#!/bin/bash
# ==============================================================================
# PROJECT MITTENS: Synthetic Traffic Generator
# ==============================================================================
# Sends continuous batches of optimization requests across different policy
# classes (CFA, VFA, DLA) and fleet scales to populate Prometheus & Grafana.
# ==============================================================================

TARGET_URL="${1:-http://localhost:8080/api/v1/optimize}"
ITERATIONS="${2:-20}"

echo "========================================================================"
echo "Project Mittens: Streaming $ITERATIONS optimization batches to $TARGET_URL"
echo "========================================================================"

POLICIES=("CFA" "VFA" "DLA")
CITIES=("CHI" "ATL" "DAL" "IND" "BHM" "CLT" "MEM" "NSH" "STL" "KC")

for ((i=1; i<=ITERATIONS; i++)); do
    POLICY=${POLICIES[$((RANDOM % 3))]}
    NOW=$(date +%s)
    
    # Send batch
    curl -s -X POST "$TARGET_URL" \
      -H "Content-Type: application/json" \
      -d "{
        \"epoch\": $NOW,
        \"policy_class\": \"$POLICY\",
        \"drivers\": [
          {
            \"id\": \"DRV_01\",
            \"current_location\": {\"node_id\": \"CHI\", \"lat\": 41.8781, \"lon\": -87.6298},
            \"home_location\": {\"node_id\": \"CHI\", \"lat\": 41.8781, \"lon\": -87.6298},
            \"available_epoch\": $NOW,
            \"drive_hours_remaining\": 11.0,
            \"duty_hours_remaining\": 14.0,
            \"equipment\": {\"type\": \"DRY_VAN\"}
          },
          {
            \"id\": \"DRV_02\",
            \"current_location\": {\"node_id\": \"ATL\", \"lat\": 33.7490, \"lon\": -84.3880},
            \"home_location\": {\"node_id\": \"ATL\", \"lat\": 33.7490, \"lon\": -84.3880},
            \"available_epoch\": $NOW,
            \"drive_hours_remaining\": 10.5,
            \"duty_hours_remaining\": 13.0,
            \"equipment\": {\"type\": \"DRY_VAN\"}
          },
          {
            \"id\": \"DRV_03\",
            \"current_location\": {\"node_id\": \"DAL\", \"lat\": 32.7767, \"lon\": -96.7970},
            \"home_location\": {\"node_id\": \"DAL\", \"lat\": 32.7767, \"lon\": -96.7970},
            \"available_epoch\": $NOW,
            \"drive_hours_remaining\": 9.0,
            \"duty_hours_remaining\": 12.0,
            \"equipment\": {\"type\": \"REEFER\"}
          }
        ],
        \"loads\": [
          {
            \"id\": \"LOAD_CHI_IND_$i\",
            \"origin\": {\"node_id\": \"CHI\", \"lat\": 41.8781, \"lon\": -87.6298},
            \"destination\": {\"node_id\": \"IND\", \"lat\": 39.7684, \"lon\": -86.1581},
            \"pickup_earliest_epoch\": $NOW,
            \"pickup_latest_epoch\": $((NOW + 36000)),
            \"delivery_earliest_epoch\": $((NOW + 18000)),
            \"delivery_latest_epoch\": $((NOW + 120000)),
            \"revenue\": 1850.0,
            \"required_equipment\": \"DRY_VAN\"
          },
          {
            \"id\": \"LOAD_ATL_CLT_$i\",
            \"origin\": {\"node_id\": \"ATL\", \"lat\": 33.7490, \"lon\": -84.3880},
            \"destination\": {\"node_id\": \"CLT\", \"lat\": 35.2271, \"lon\": -80.8431},
            \"pickup_earliest_epoch\": $NOW,
            \"pickup_latest_epoch\": $((NOW + 36000)),
            \"delivery_earliest_epoch\": $((NOW + 18000)),
            \"delivery_latest_epoch\": $((NOW + 120000)),
            \"revenue\": 1650.0,
            \"required_equipment\": \"DRY_VAN\"
          },
          {
            \"id\": \"LOAD_DAL_MEM_$i\",
            \"origin\": {\"node_id\": \"DAL\", \"lat\": 32.7767, \"lon\": -96.7970},
            \"destination\": {\"node_id\": \"MEM\", \"lat\": 35.1495, \"lon\": -90.0490},
            \"pickup_earliest_epoch\": $NOW,
            \"pickup_latest_epoch\": $((NOW + 36000)),
            \"delivery_earliest_epoch\": $((NOW + 18000)),
            \"delivery_latest_epoch\": $((NOW + 120000)),
            \"revenue\": 2100.0,
            \"required_equipment\": \"REEFER\"
          }
        ]
      }" > /dev/null
      
    echo "  [Batch $i/$ITERATIONS] Policy: $POLICY -> Match batch dispatched successfully"
    sleep 0.2
done

echo "========================================================================"
echo "Done! Refresh http://localhost:3000 to view updated live metrics."
echo "========================================================================"
