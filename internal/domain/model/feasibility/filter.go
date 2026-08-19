package feasibility

import (
	"context"
	"fmt"
	"log/slog"
	"runtime"
	"sort"
	"sync"
	"time"

	"github.com/optimaldynamics/project-mittens/internal/domain/model"
	"github.com/optimaldynamics/project-mittens/internal/domain/model/hos"
)

// CandidateArc represents a physically feasible match assignment between a driver and a customer load.
type CandidateArc struct {
	DriverID            string
	LoadID              string
	DeadheadMiles       float64
	LoadedMiles         float64
	DeadheadDriveMin    int
	LoadedDriveMin      int
	InsertedRestMin     int
	InsertedDwellMin    int
	TotalTripMin        int
	PickupArrivalTime   time.Time
	LoadingEndTime      time.Time
	DeliveryArrivalTime time.Time
	UnloadingEndTime    time.Time
}

// FilterConfig holds operational settings for the concurrent feasibility evaluation.
type FilterConfig struct {
	Feasibility model.FeasibilityConfig
	WorkerCount int // Number of parallel worker goroutines (defaults to runtime.NumCPU())
}

// ConcurrentFilter evaluates the full Cartesian driver-load candidate matrix in parallel,
// pruning physically infeasible pairings and returning a canonically sorted slice of CandidateArcs.
//
// In accordance with Inviolate 5 (Immutability), Inviolate 6 (Lock-Free Hot Paths), and
// Principle 2 (Deterministic Reproducibility):
//   - Work is partitioned across worker goroutines via lock-free channel multiplexing.
//   - All workers select on ctx.Done() to ensure zero goroutine leaks on timeout.
//   - Output arcs are canonically sorted by (DriverID, LoadID) to guarantee bit-wise identical output.
type ConcurrentFilter struct {
	sim *hos.Simulator
}

// NewConcurrentFilter returns a new ConcurrentFilter instance.
func NewConcurrentFilter() *ConcurrentFilter {
	return &ConcurrentFilter{
		sim: hos.NewSimulator(),
	}
}

type driverJob struct {
	driver model.Driver
}

type workerResult struct {
	arcs []CandidateArc
	err  error
}

// FilterCandidates evaluates all candidate pairings across drivers and loads concurrently.
func (f *ConcurrentFilter) FilterCandidates(
	ctx context.Context,
	drivers []model.Driver,
	loads []model.Load,
	cfg FilterConfig,
) ([]CandidateArc, error) {
	if len(drivers) == 0 || len(loads) == 0 {
		return nil, nil
	}

	workerCount := cfg.WorkerCount
	if workerCount <= 0 {
		workerCount = runtime.NumCPU()
	}
	if workerCount > len(drivers) {
		workerCount = len(drivers)
	}

	jobsChan := make(chan driverJob, len(drivers))
	resultsChan := make(chan workerResult, workerCount)

	// Feed driver jobs into channel
	for _, d := range drivers {
		jobsChan <- driverJob{driver: d}
	}
	close(jobsChan)

	var wg sync.WaitGroup

	// Launch worker pool
	for w := 0; w < workerCount; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			localArcs := make([]CandidateArc, 0, len(loads))

			for job := range jobsChan {
				select {
				case <-ctx.Done():
					resultsChan <- workerResult{err: ctx.Err()}
					return
				default:
				}

				d := job.driver
				for _, l := range loads {
					arc, isFeasible, err := f.evaluatePair(d, l, cfg.Feasibility)
					if err != nil {
						resultsChan <- workerResult{err: err}
						return
					}
					if isFeasible {
						localArcs = append(localArcs, arc)
					}
				}
			}

			select {
			case resultsChan <- workerResult{arcs: localArcs}:
			case <-ctx.Done():
			}
		}()
	}

	// Wait for all workers in a coordinator goroutine and close results channel
	go func() {
		wg.Wait()
		close(resultsChan)
	}()

	var allArcs []CandidateArc
	for res := range resultsChan {
		if res.err != nil {
			return nil, res.err
		}
		allArcs = append(allArcs, res.arcs...)
	}

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	// Sort canonically by DriverID, then LoadID to guarantee determinism (Principle 2)
	sort.Slice(allArcs, func(i, j int) bool {
		if allArcs[i].DriverID != allArcs[j].DriverID {
			return allArcs[i].DriverID < allArcs[j].DriverID
		}
		return allArcs[i].LoadID < allArcs[j].LoadID
	})

	return allArcs, nil
}

func (f *ConcurrentFilter) evaluatePair(
	driver model.Driver,
	load model.Load,
	cfg model.FeasibilityConfig,
) (CandidateArc, bool, error) {
	// 1. Equipment & Endorsement compatibility check
	if !driver.Equipment.CanHandle(load.RequiredEquipment, load.RequiredEndorsements) {
		slog.Debug("candidate arc rejected: equipment incompatible",
			slog.String("driver_id", driver.ID),
			slog.String("load_id", load.ID),
			slog.String("driver_equipment", string(driver.Equipment.Type)),
			slog.String("required_equipment", string(load.RequiredEquipment)),
		)
		return CandidateArc{}, false, nil
	}

	// 2. Deadhead distance check
	deadheadMiles := driver.CurrentLocation.DistanceMiles(load.Origin)
	if cfg.MaxDeadheadMiles > 0 && deadheadMiles > cfg.MaxDeadheadMiles {
		slog.Debug("candidate arc rejected: deadhead limit exceeded",
			slog.String("driver_id", driver.ID),
			slog.String("load_id", load.ID),
			slog.Float64("deadhead_miles", deadheadMiles),
			slog.Float64("max_deadhead", cfg.MaxDeadheadMiles),
		)
		return CandidateArc{}, false, nil
	}

	// 3. Linehaul distance
	loadedMiles := load.Origin.DistanceMiles(load.Destination)

	// 4. Driver Clocks & Policy setup with safe fallback
	specs := driver.PolicySpecs
	if specs.Name == "" {
		specs = cfg.HOSPolicySpecs
	}
	if specs.Name == "" {
		specs = hos.USPolicySpecs()
	}

	clocks := driver.Clocks
	if clocks == nil {
		startTime := time.Unix(driver.AvailableEpoch, 0).UTC()
		clocks = hos.NewDriverClocks(specs, startTime)
	}

	// 5. Convert load epoch windows to time.Time
	var pickupEarliest, pickupLatest, deliveryEarliest, deliveryLatest time.Time
	if load.PickupEarliestEpoch > 0 {
		pickupEarliest = time.Unix(load.PickupEarliestEpoch, 0).UTC()
	}
	if load.PickupLatestEpoch > 0 {
		pickupLatest = time.Unix(load.PickupLatestEpoch, 0).UTC()
	}
	if load.DeliveryEarliestEpoch > 0 {
		deliveryEarliest = time.Unix(load.DeliveryEarliestEpoch, 0).UTC()
	}
	if load.DeliveryLatestEpoch > 0 {
		deliveryLatest = time.Unix(load.DeliveryLatestEpoch, 0).UTC()
	}

	// 6. Evaluate HOS and time window feasibility via HOS simulator
	tripRes, err := f.sim.EvaluateTripFeasibility(
		clocks,
		deadheadMiles,
		loadedMiles,
		60, 60, // Standard 60 min loading and unloading dwell
		cfg.AverageSpeedMPH,
		pickupEarliest,
		pickupLatest,
		deliveryEarliest,
		deliveryLatest,
		specs,
	)
	if err != nil {
		return CandidateArc{}, false, fmt.Errorf("feasibility: trip evaluation failed for %s -> %s: %w", driver.ID, load.ID, err)
	}

	if !tripRes.IsFeasible {
		slog.Debug("candidate arc rejected: trip infeasible",
			slog.String("driver_id", driver.ID),
			slog.String("load_id", load.ID),
			slog.String("reason", tripRes.InfeasibilityReason),
		)
		return CandidateArc{}, false, nil
	}

	arc := CandidateArc{
		DriverID:            driver.ID,
		LoadID:              load.ID,
		DeadheadMiles:       deadheadMiles,
		LoadedMiles:         loadedMiles,
		DeadheadDriveMin:    tripRes.DeadheadDriveMin,
		LoadedDriveMin:      tripRes.LoadedDriveMin,
		InsertedRestMin:     tripRes.InsertedRestMin,
		InsertedDwellMin:    tripRes.InsertedDwellMin,
		TotalTripMin:        tripRes.TotalTripDurationMin,
		PickupArrivalTime:   tripRes.PickupArrivalTime,
		LoadingEndTime:      tripRes.LoadingEndTime,
		DeliveryArrivalTime: tripRes.DeliveryArrivalTime,
		UnloadingEndTime:    tripRes.UnloadingEndTime,
	}

	return arc, true, nil
}
