package boilerplate

import (
	"embed"
	"reflect"
	"testing"
)

func TestGetProjectFs(t *testing.T) {
	for _, tc := range []struct {
		Name    string
		Input   string
		Want    embed.FS
		WantErr bool
	}{
		{
			Name:    "Valid Cobra Project",
			Input:   "cobra",
			Want:    cobraProject,
			WantErr: false,
		},
		{
			Name:    "Invalid",
			Input:   "invalid",
			WantErr: true,
		},
	} {
		t.Run(tc.Name, func(t *testing.T) {
			fs, _, err := GetProjectFs(tc.Input)
			if err != nil {
				if !tc.WantErr {
					t.Errorf("unexpected err occured: %v", err)
				}
				return
			}
			if reflect.DeepEqual(fs, embed.FS{}) {
				t.Errorf("fs nil unexpected")
			} else if !reflect.DeepEqual(fs, tc.Want) {

			}
		})
	}
}
