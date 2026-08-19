package model

import (
	"errors"
	"fmt"
	"math"
)

var (
	// ErrInvalidInformationState is returned when macro indicators violate non-negative bounds.
	ErrInvalidInformationState = errors.New("domain/model: invalid information state values")
)

// InformationState (I_t) represents observable non-resource macro variables in the transportation network
// at discrete decision epoch t (such as diesel fuel prices, aggregate spot market rate indices, and weather severity).
//
// In accordance with Inviolate 0, Inviolate 2, and Inviolate 5, InformationState is completely immutable once allocated.
type InformationState struct {
	epoch                 int64
	fuelPriceIndex        float64 // Normalized diesel fuel cost index (must be >= 0)
	nationalSpotRateIndex float64 // National freight linehaul spot index ($/mile, must be >= 0)
	weatherAlertCount     int     // Number of active severe weather lane alerts (must be >= 0)
}

// NewInformationState creates and validates an immutable InformationState instance.
// Returns an error if any numerical index is negative or non-finite.
func NewInformationState(epoch int64, fuelPriceIndex, spotRateIndex float64, weatherAlertCount int) (*InformationState, error) {
	if epoch < 0 {
		return nil, fmt.Errorf("%w: epoch %d must be >= 0", ErrInvalidInformationState, epoch)
	}
	if fuelPriceIndex < 0 || math.IsNaN(fuelPriceIndex) || math.IsInf(fuelPriceIndex, 0) {
		return nil, fmt.Errorf("%w: fuelPriceIndex %f must be finite and >= 0", ErrInvalidInformationState, fuelPriceIndex)
	}
	if spotRateIndex < 0 || math.IsNaN(spotRateIndex) || math.IsInf(spotRateIndex, 0) {
		return nil, fmt.Errorf("%w: spotRateIndex %f must be finite and >= 0", ErrInvalidInformationState, spotRateIndex)
	}
	if weatherAlertCount < 0 {
		return nil, fmt.Errorf("%w: weatherAlertCount %d must be >= 0", ErrInvalidInformationState, weatherAlertCount)
	}

	return &InformationState{
		epoch:                 epoch,
		fuelPriceIndex:        fuelPriceIndex,
		nationalSpotRateIndex: spotRateIndex,
		weatherAlertCount:     weatherAlertCount,
	}, nil
}

// Epoch returns the discrete decision epoch index.
func (is *InformationState) Epoch() int64 {
	return is.epoch
}

// FuelPriceIndex returns the normalized fuel price index.
func (is *InformationState) FuelPriceIndex() float64 {
	return is.fuelPriceIndex
}

// NationalSpotRateIndex returns the national spot linehaul freight rate index ($/mile).
func (is *InformationState) NationalSpotRateIndex() float64 {
	return is.nationalSpotRateIndex
}

// WeatherAlertCount returns the count of active severe weather lane warnings.
func (is *InformationState) WeatherAlertCount() int {
	return is.weatherAlertCount
}

// Clone returns an exact copy of the InformationState (Inviolate 5).
func (is *InformationState) Clone() *InformationState {
	return &InformationState{
		epoch:                 is.epoch,
		fuelPriceIndex:        is.fuelPriceIndex,
		nationalSpotRateIndex: is.nationalSpotRateIndex,
		weatherAlertCount:     is.weatherAlertCount,
	}
}

// Transition returns a newly allocated InformationState advanced to the next epoch with updated macro signals.
func (is *InformationState) Transition(nextEpoch int64, fuelIndex, spotIndex float64, weatherAlerts int) (*InformationState, error) {
	if nextEpoch <= is.epoch {
		return nil, fmt.Errorf("%w: nextEpoch %d must be strictly greater than current epoch %d", ErrInvalidInformationState, nextEpoch, is.epoch)
	}
	return NewInformationState(nextEpoch, fuelIndex, spotIndex, weatherAlerts)
}
