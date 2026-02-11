package dbt

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSelectVersionsForDeletion(t *testing.T) {
	now := time.Now()

	versions := []VersionInfo{
		{Version: "1.0.0", ModifiedAt: now.Add(-90 * 24 * time.Hour)}, // 90 days old
		{Version: "1.1.0", ModifiedAt: now.Add(-60 * 24 * time.Hour)}, // 60 days old
		{Version: "2.0.0", ModifiedAt: now.Add(-30 * 24 * time.Hour)}, // 30 days old
		{Version: "2.1.0", ModifiedAt: now.Add(-7 * 24 * time.Hour)},  // 7 days old
		{Version: "3.0.0", ModifiedAt: now.Add(-1 * 24 * time.Hour)},  // 1 day old
	}

	tests := []struct {
		name          string
		opts          PurgeOptions
		expectedCount int
		expectedNames []string
	}{
		{
			name: "older than 45 days",
			opts: PurgeOptions{
				OlderThan: 45 * 24 * time.Hour,
			},
			expectedCount: 2,
			expectedNames: []string{"1.0.0", "1.1.0"},
		},
		{
			name: "older than 45 days with keep 4",
			opts: PurgeOptions{
				OlderThan: 45 * 24 * time.Hour,
				Keep:      4,
			},
			expectedCount: 1,
			expectedNames: []string{"1.0.0"},
		},
		{
			name: "keep 3 no age filter",
			opts: PurgeOptions{
				Keep: 3,
			},
			expectedCount: 2,
			expectedNames: []string{"1.0.0", "1.1.0"},
		},
		{
			name: "keep 1 (keep latest)",
			opts: PurgeOptions{
				Keep: 1,
			},
			expectedCount: 4,
		},
		{
			name: "keep more than total",
			opts: PurgeOptions{
				Keep: 10,
			},
			expectedCount: 0,
		},
		{
			name: "older than 1 year - nothing matches",
			opts: PurgeOptions{
				OlderThan: 365 * 24 * time.Hour,
			},
			expectedCount: 0,
		},
		{
			name:          "no filters selects all",
			opts:          PurgeOptions{},
			expectedCount: 5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := selectVersionsForDeletion(versions, tt.opts)
			assert.Len(t, result, tt.expectedCount)

			if tt.expectedNames != nil {
				resultNames := make([]string, len(result))
				for i, v := range result {
					resultNames[i] = v.Version
				}
				for _, expectedName := range tt.expectedNames {
					assert.Contains(t, resultNames, expectedName)
				}
			}
		})
	}
}

func TestParseDuration(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected time.Duration
		wantErr  bool
	}{
		{
			name:     "30 days",
			input:    "30d",
			expected: 30 * 24 * time.Hour,
			wantErr:  false,
		},
		{
			name:     "2 weeks",
			input:    "2w",
			expected: 2 * 7 * 24 * time.Hour,
			wantErr:  false,
		},
		{
			name:     "24 hours",
			input:    "24h",
			expected: 24 * time.Hour,
			wantErr:  false,
		},
		{
			name:     "90 minutes",
			input:    "90m",
			expected: 90 * time.Minute,
			wantErr:  false,
		},
		{
			name:    "invalid format",
			input:   "abc",
			wantErr: true,
		},
		{
			name:    "invalid day value",
			input:   "abcd",
			wantErr: true,
		},
		{
			name:    "invalid week value",
			input:   "xyzw",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseDuration(tt.input)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestSelectVersionsForDeletionSortOrder(t *testing.T) {
	now := time.Now()

	// Versions in random order - should be sorted by semver (newest first)
	versions := []VersionInfo{
		{Version: "1.5.0", ModifiedAt: now.Add(-10 * 24 * time.Hour)},
		{Version: "2.0.0", ModifiedAt: now.Add(-5 * 24 * time.Hour)},
		{Version: "1.0.0", ModifiedAt: now.Add(-20 * 24 * time.Hour)},
	}

	opts := PurgeOptions{
		Keep: 1, // Keep only the newest
	}

	result := selectVersionsForDeletion(versions, opts)
	assert.Len(t, result, 2)

	// The newest version (2.0.0) should NOT be in the delete list
	resultNames := make([]string, len(result))
	for i, v := range result {
		resultNames[i] = v.Version
	}
	assert.NotContains(t, resultNames, "2.0.0", "newest version should be retained")
	assert.Contains(t, resultNames, "1.0.0")
	assert.Contains(t, resultNames, "1.5.0")
}

func TestPurgeToolWarnsOnFullDelete(t *testing.T) {
	// This test verifies the selectVersionsForDeletion correctly identifies
	// when all versions would be deleted
	now := time.Now()

	versions := []VersionInfo{
		{Version: "1.0.0", ModifiedAt: now.Add(-90 * 24 * time.Hour)},
		{Version: "2.0.0", ModifiedAt: now.Add(-60 * 24 * time.Hour)},
	}

	opts := PurgeOptions{
		OlderThan: 1 * 24 * time.Hour, // Everything is older than 1 day
	}

	result := selectVersionsForDeletion(versions, opts)
	assert.Len(t, result, len(versions), "all versions should be selected for deletion")
}
