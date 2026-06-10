package config

import "testing"

func TestDefaultValidates(t *testing.T) {
	c := Default()
	if err := c.Validate(); err != nil {
		t.Fatalf("default config should validate: %v", err)
	}
	if c.Width != 1024 || c.Height != 1024 || c.Probability != 0.10 {
		t.Fatalf("unexpected defaults: %+v", c)
	}
}

func TestValidateClampsProbability(t *testing.T) {
	c := Default()
	c.Probability = 5
	if err := c.Validate(); err != nil {
		t.Fatal(err)
	}
	if c.Probability != 1 {
		t.Fatalf("probability = %v, want clamped to 1", c.Probability)
	}
}

func TestValidateRejectsBadTickHz(t *testing.T) {
	c := Default()
	c.TickHz = 0
	if err := c.Validate(); err == nil {
		t.Fatal("expected error for TickHz=0")
	}
}

func TestValidateSizeBounds(t *testing.T) {
	c := Default()
	if err := c.ValidateSize(c.MinDim, c.MinDim); err != nil {
		t.Fatalf("min size should be valid: %v", err)
	}
	if err := c.ValidateSize(c.MinDim-1, c.MinDim); err == nil {
		t.Fatal("below-min width should fail")
	}
	if err := c.ValidateSize(c.MaxDim+1, c.MinDim); err == nil {
		t.Fatal("above-max width should fail")
	}

	c.MaxCells = 1000
	if err := c.ValidateSize(100, 100); err == nil {
		t.Fatal("over-max-cells should fail")
	}
}

func TestClampProbability(t *testing.T) {
	cases := map[float64]float64{-1: 0, 0: 0, 0.5: 0.5, 1: 1, 2: 1}
	for in, want := range cases {
		if got := ClampProbability(in); got != want {
			t.Errorf("ClampProbability(%v) = %v, want %v", in, got, want)
		}
	}
}

func TestEffectiveStreamEveryN(t *testing.T) {
	c := Default()
	if got := c.EffectiveStreamEveryN(); got != 1 {
		t.Fatalf("5Hz under 30fps cap = %d, want 1", got)
	}
	c.TickHz = 50
	if got := c.EffectiveStreamEveryN(); got != 2 {
		t.Fatalf("50Hz capped at 30fps = %d, want 2", got)
	}
	c.StreamEveryN = 5
	if got := c.EffectiveStreamEveryN(); got != 5 {
		t.Fatalf("explicit streamEveryN = %d, want 5", got)
	}
}
