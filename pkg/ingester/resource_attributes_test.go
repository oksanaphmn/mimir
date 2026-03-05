// SPDX-License-Identifier: AGPL-3.0-only

package ingester

import (
	"testing"

	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/tsdb/seriesmetadata"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildResourceAttributesItem(t *testing.T) {
	tests := []struct {
		name             string
		versions         []*seriesmetadata.ResourceVersion
		startMs          int64
		endMs            int64
		expectedCount    int // -1 means expect nil
		expectedMinTimes []int64
	}{
		{
			name: "version fully within range",
			versions: []*seriesmetadata.ResourceVersion{
				{MinTime: 1000, MaxTime: 2000, Identifying: map[string]string{"service.name": "test"}},
			},
			startMs:          1000,
			endMs:            2000,
			expectedCount:    1,
			expectedMinTimes: []int64{1000},
		},
		{
			name: "version before range excluded",
			versions: []*seriesmetadata.ResourceVersion{
				{MinTime: 500, MaxTime: 999, Identifying: map[string]string{"service.name": "test"}},
			},
			startMs:       1000,
			endMs:         2000,
			expectedCount: -1,
		},
		{
			name: "version after range excluded",
			versions: []*seriesmetadata.ResourceVersion{
				{MinTime: 2001, MaxTime: 3000, Identifying: map[string]string{"service.name": "test"}},
			},
			startMs:       1000,
			endMs:         2000,
			expectedCount: -1,
		},
		{
			name: "version overlaps start of range included",
			versions: []*seriesmetadata.ResourceVersion{
				{MinTime: 500, MaxTime: 1500, Identifying: map[string]string{"service.name": "test"}},
			},
			startMs:          1000,
			endMs:            2000,
			expectedCount:    1,
			expectedMinTimes: []int64{500},
		},
		{
			name: "version overlaps end of range included",
			versions: []*seriesmetadata.ResourceVersion{
				{MinTime: 1500, MaxTime: 2500, Identifying: map[string]string{"service.name": "test"}},
			},
			startMs:          1000,
			endMs:            2000,
			expectedCount:    1,
			expectedMinTimes: []int64{1500},
		},
		{
			name: "open start (startMs=0) includes all versions before endMs",
			versions: []*seriesmetadata.ResourceVersion{
				{MinTime: 100, MaxTime: 500, Identifying: map[string]string{"service.name": "early"}},
				{MinTime: 1000, MaxTime: 2000, Identifying: map[string]string{"service.name": "mid"}},
				{MinTime: 5000, MaxTime: 6000, Identifying: map[string]string{"service.name": "late"}},
			},
			startMs:          0,
			endMs:            3000,
			expectedCount:    2,
			expectedMinTimes: []int64{100, 1000},
		},
		{
			name: "open end (endMs=0) includes all versions after startMs",
			versions: []*seriesmetadata.ResourceVersion{
				{MinTime: 100, MaxTime: 500, Identifying: map[string]string{"service.name": "early"}},
				{MinTime: 1000, MaxTime: 2000, Identifying: map[string]string{"service.name": "mid"}},
				{MinTime: 5000, MaxTime: 6000, Identifying: map[string]string{"service.name": "late"}},
			},
			startMs:          800,
			endMs:            0,
			expectedCount:    2,
			expectedMinTimes: []int64{1000, 5000},
		},
		{
			name: "both open (startMs=0, endMs=0) includes all versions",
			versions: []*seriesmetadata.ResourceVersion{
				{MinTime: 100, MaxTime: 500, Identifying: map[string]string{"service.name": "early"}},
				{MinTime: 1000, MaxTime: 2000, Identifying: map[string]string{"service.name": "mid"}},
			},
			startMs:          0,
			endMs:            0,
			expectedCount:    2,
			expectedMinTimes: []int64{100, 1000},
		},
		{
			name: "no versions match returns nil",
			versions: []*seriesmetadata.ResourceVersion{
				{MinTime: 100, MaxTime: 200},
				{MinTime: 300, MaxTime: 400},
			},
			startMs:       500,
			endMs:         600,
			expectedCount: -1,
		},
		{
			name: "entities correctly copied",
			versions: []*seriesmetadata.ResourceVersion{
				{
					MinTime:     1000,
					MaxTime:     2000,
					Identifying: map[string]string{"service.name": "test"},
					Entities: []*seriesmetadata.Entity{
						{
							Type:        "service",
							ID:          map[string]string{"service.name": "test"},
							Description: map[string]string{"service.version": "1.0"},
						},
					},
				},
			},
			startMs:          1000,
			endMs:            2000,
			expectedCount:    1,
			expectedMinTimes: []int64{1000},
		},
	}

	lbls := labels.FromStrings("__name__", "test_metric", "job", "test")

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			vr := &seriesmetadata.VersionedResource{Versions: tc.versions}
			result := buildResourceAttributesItem(lbls, vr, tc.startMs, tc.endMs)

			if tc.expectedCount == -1 {
				assert.Nil(t, result, "expected nil result when no versions match")
				return
			}

			require.NotNil(t, result)
			require.Len(t, result.Versions, tc.expectedCount)

			for i, expected := range tc.expectedMinTimes {
				assert.Equal(t, expected, result.Versions[i].MinTimeMs)
			}

			// For the entities test case, verify entity data is deep-copied.
			if tc.name == "entities correctly copied" {
				require.Len(t, result.Versions[0].Entities, 1)
				assert.Equal(t, "service", result.Versions[0].Entities[0].Type)
				assert.Equal(t, "test", result.Versions[0].Entities[0].Id["service.name"])
				assert.Equal(t, "1.0", result.Versions[0].Entities[0].Description["service.version"])
			}
		})
	}
}

func TestBuildResourceAttributesItem_DeepCopy(t *testing.T) {
	// Verify that modifying the source data after building doesn't affect the result.
	identifying := map[string]string{"service.name": "original"}
	descriptive := map[string]string{"service.version": "1.0"}
	entityID := map[string]string{"service.name": "original"}
	entityDesc := map[string]string{"service.version": "1.0"}
	vr := &seriesmetadata.VersionedResource{
		Versions: []*seriesmetadata.ResourceVersion{
			{
				MinTime:     1000,
				MaxTime:     2000,
				Identifying: identifying,
				Descriptive: descriptive,
				Entities: []*seriesmetadata.Entity{
					{Type: "service", ID: entityID, Description: entityDesc},
				},
			},
		},
	}

	lbls := labels.FromStrings("__name__", "test_metric")
	result := buildResourceAttributesItem(lbls, vr, 0, 0)
	require.NotNil(t, result)

	// Mutate all source maps.
	identifying["service.name"] = "mutated"
	descriptive["service.version"] = "mutated"
	entityID["service.name"] = "mutated"
	entityDesc["service.version"] = "mutated"

	// Result should still have the original values (deep copy).
	assert.Equal(t, "original", result.Versions[0].Identifying["service.name"])
	assert.Equal(t, "1.0", result.Versions[0].Descriptive["service.version"])
	assert.Equal(t, "original", result.Versions[0].Entities[0].Id["service.name"])
	assert.Equal(t, "1.0", result.Versions[0].Entities[0].Description["service.version"])
}

func TestIntersectSortedUint64(t *testing.T) {
	tests := []struct {
		name     string
		a        []uint64
		b        []uint64
		expected []uint64
	}{
		{
			name:     "both empty",
			a:        nil,
			b:        nil,
			expected: nil,
		},
		{
			name:     "a empty",
			a:        nil,
			b:        []uint64{1, 2, 3},
			expected: nil,
		},
		{
			name:     "b empty",
			a:        []uint64{1, 2, 3},
			b:        nil,
			expected: nil,
		},
		{
			name:     "no intersection",
			a:        []uint64{1, 3, 5},
			b:        []uint64{2, 4, 6},
			expected: nil,
		},
		{
			name:     "full overlap",
			a:        []uint64{1, 2, 3},
			b:        []uint64{1, 2, 3},
			expected: []uint64{1, 2, 3},
		},
		{
			name:     "partial overlap",
			a:        []uint64{1, 2, 3, 4, 5},
			b:        []uint64{3, 4, 5, 6, 7},
			expected: []uint64{3, 4, 5},
		},
		{
			name:     "single element match",
			a:        []uint64{1, 5, 10},
			b:        []uint64{5},
			expected: []uint64{5},
		},
		{
			name:     "disjoint with large gap",
			a:        []uint64{1, 2, 3},
			b:        []uint64{100, 200, 300},
			expected: nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := intersectSortedUint64(tc.a, tc.b)
			if tc.expected == nil {
				// intersectSortedUint64 returns nil for empty cases, but may
				// return an empty non-nil slice for no-intersection of non-empty inputs.
				assert.Empty(t, result)
			} else {
				assert.Equal(t, tc.expected, result)
			}
		})
	}
}
