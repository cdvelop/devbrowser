package devbrowser_test

import (
	"webtyp.com/devbrowser"
	"testing"
)

func TestCalculateConstrainedSize(t *testing.T) {
	b := &devbrowser.DevBrowser{}

	tests := []struct {
		name  string
		reqW  int
		reqH  int
		monW  int
		monH  int
		wantW int
		wantH int
	}{
		{
			name:  "Fits comfortably",
			reqW:  1024,
			reqH:  768,
			monW:  1920,
			monH:  1080,
			wantW: 1024,
			wantH: 768,
		},
		{
			name:  "Too wide",
			reqW:  2000,
			reqH:  800,
			monW:  1920,
			monH:  1080,
			wantW: 1920,
			wantH: 800,
		},
		{
			name:  "Too tall",
			reqW:  800,
			reqH:  1200,
			monW:  1920,
			monH:  1080,
			wantW: 800,
			wantH: 1080,
		},
		{
			name:  "Too wide and tall",
			reqW:  2560,
			reqH:  1440,
			monW:  1920,
			monH:  1080,
			wantW: 1920,
			wantH: 1080,
		},
		{
			name:  "Zero monitor size (unknown)",
			reqW:  1024,
			reqH:  768,
			monW:  0,
			monH:  0,
			wantW: 1024, // Should return requested
			wantH: 768,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotW, gotH := b.CalculateConstrainedSize(tt.reqW, tt.reqH, tt.monW, tt.monH)
			if gotW != tt.wantW || gotH != tt.wantH {
				t.Errorf("calculateConstrainedSize() = %v, %v, want %v, %v", gotW, gotH, tt.wantW, tt.wantH)
			}
		})
	}
}
