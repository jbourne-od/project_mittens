package db

import (
	"context"
	"fmt"

	"github.com/optimaldynamics/project-mittens/internal/domain/model"
)

// PostgresFleetRepository handles persistence and querying of physical fleet resources (drivers and loads).
type PostgresFleetRepository struct {
	pool *Pool
}

// NewPostgresFleetRepository initializes a new PostgresFleetRepository.
func NewPostgresFleetRepository(pool *Pool) *PostgresFleetRepository {
	return &PostgresFleetRepository{pool: pool}
}

// UpsertDrivers commits a slice of drivers to PostgreSQL.
func (r *PostgresFleetRepository) UpsertDrivers(ctx context.Context, drivers []model.Driver) error {
	if len(drivers) == 0 {
		return nil
	}

	query := `
		INSERT INTO fleet_drivers (
			driver_id, current_node, home_node, current_lat, current_lon,
			available_epoch, drive_hours_remaining, duty_hours_remaining,
			assigned_load_id, equipment_type, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, NOW())
		ON CONFLICT (driver_id) DO UPDATE SET
			current_node = EXCLUDED.current_node,
			home_node = EXCLUDED.home_node,
			current_lat = EXCLUDED.current_lat,
			current_lon = EXCLUDED.current_lon,
			available_epoch = EXCLUDED.available_epoch,
			drive_hours_remaining = EXCLUDED.drive_hours_remaining,
			duty_hours_remaining = EXCLUDED.duty_hours_remaining,
			assigned_load_id = EXCLUDED.assigned_load_id,
			equipment_type = EXCLUDED.equipment_type,
			updated_at = NOW();
	`

	for _, d := range drivers {
		equipType := string(d.Equipment.Type)
		if equipType == "" {
			equipType = string(model.EquipDryVan)
		}

		_, err := r.pool.Exec(ctx, query,
			d.ID, d.CurrentLocation.NodeID, d.HomeLocation.NodeID,
			d.CurrentLocation.Lat, d.CurrentLocation.Lon,
			d.AvailableEpoch, d.DriveHoursRemaining,
			d.DutyHoursRemaining, d.AssignedLoadID, equipType,
		)
		if err != nil {
			return fmt.Errorf("db: failed upserting driver %s: %w", d.ID, err)
		}
	}

	return nil
}

// GetDrivers retrieves all active drivers from PostgreSQL.
func (r *PostgresFleetRepository) GetDrivers(ctx context.Context) ([]model.Driver, error) {
	query := `
		SELECT driver_id, current_node, home_node, current_lat, current_lon,
		       available_epoch, drive_hours_remaining, duty_hours_remaining,
		       assigned_load_id, equipment_type
		FROM fleet_drivers
		ORDER BY driver_id ASC;
	`

	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("db: failed querying drivers: %w", err)
	}
	defer rows.Close()

	var drivers []model.Driver
	for rows.Next() {
		var (
			dID, currNode, homeNode, assignedLoadID, equipStr string
			currLat, currLon                                  float64
			availEpoch                                        int64
			driveH, dutyH                                     float64
		)

		err := rows.Scan(
			&dID, &currNode, &homeNode, &currLat, &currLon,
			&availEpoch, &driveH, &dutyH, &assignedLoadID, &equipStr,
		)
		if err != nil {
			return nil, fmt.Errorf("db: failed scanning driver row: %w", err)
		}

		drivers = append(drivers, model.Driver{
			ID: dID,
			CurrentLocation: model.Location{
				NodeID: currNode,
				Lat:    currLat,
				Lon:    currLon,
			},
			HomeLocation: model.Location{
				NodeID: homeNode,
			},
			AvailableEpoch:      availEpoch,
			DriveHoursRemaining: driveH,
			DutyHoursRemaining:  dutyH,
			AssignedLoadID:      assignedLoadID,
			Equipment: model.Equipment{
				Type: model.EquipmentType(equipStr),
			},
		})
	}

	return drivers, rows.Err()
}

// UpsertLoads commits a batch of load tenders to PostgreSQL.
func (r *PostgresFleetRepository) UpsertLoads(ctx context.Context, loads []model.Load) error {
	if len(loads) == 0 {
		return nil
	}

	query := `
		INSERT INTO fleet_loads (
			load_id, origin_node, dest_node, pickup_earliest_epoch, pickup_latest_epoch,
			delivery_earliest_epoch, delivery_latest_epoch, revenue, required_equipment, status, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, 'AVAILABLE', NOW())
		ON CONFLICT (load_id) DO UPDATE SET
			origin_node = EXCLUDED.origin_node,
			dest_node = EXCLUDED.dest_node,
			pickup_earliest_epoch = EXCLUDED.pickup_earliest_epoch,
			pickup_latest_epoch = EXCLUDED.pickup_latest_epoch,
			delivery_earliest_epoch = EXCLUDED.delivery_earliest_epoch,
			delivery_latest_epoch = EXCLUDED.delivery_latest_epoch,
			revenue = EXCLUDED.revenue,
			required_equipment = EXCLUDED.required_equipment;
	`

	for _, l := range loads {
		equipType := string(l.RequiredEquipment)
		if equipType == "" {
			equipType = string(model.EquipDryVan)
		}

		_, err := r.pool.Exec(ctx, query,
			l.ID, l.Origin.NodeID, l.Destination.NodeID,
			l.PickupEarliestEpoch, l.PickupLatestEpoch,
			l.DeliveryEarliestEpoch, l.DeliveryLatestEpoch,
			l.Revenue, equipType,
		)
		if err != nil {
			return fmt.Errorf("db: failed upserting load %s: %w", l.ID, err)
		}
	}

	return nil
}

// GetAvailableLoads retrieves all available load tenders.
func (r *PostgresFleetRepository) GetAvailableLoads(ctx context.Context) ([]model.Load, error) {
	query := `
		SELECT load_id, origin_node, dest_node, pickup_earliest_epoch, pickup_latest_epoch,
		       delivery_earliest_epoch, delivery_latest_epoch, revenue, required_equipment
		FROM fleet_loads
		WHERE status = 'AVAILABLE'
		ORDER BY pickup_earliest_epoch ASC;
	`

	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("db: failed querying available loads: %w", err)
	}
	defer rows.Close()

	var loads []model.Load
	for rows.Next() {
		var (
			lID, origNode, destNode, equipStr                    string
			pickupEarly, pickupLate, deliveryEarly, deliveryLate int64
			rev                                                  float64
		)

		err := rows.Scan(
			&lID, &origNode, &destNode,
			&pickupEarly, &pickupLate, &deliveryEarly, &deliveryLate,
			&rev, &equipStr,
		)
		if err != nil {
			return nil, fmt.Errorf("db: failed scanning load row: %w", err)
		}

		loads = append(loads, model.Load{
			ID:                    lID,
			Origin:                model.Location{NodeID: origNode},
			Destination:           model.Location{NodeID: destNode},
			PickupEarliestEpoch:   pickupEarly,
			PickupLatestEpoch:     pickupLate,
			DeliveryEarliestEpoch: deliveryEarly,
			DeliveryLatestEpoch:   deliveryLate,
			Revenue:               rev,
			RequiredEquipment:     model.EquipmentType(equipStr),
		})
	}

	return loads, rows.Err()
}
