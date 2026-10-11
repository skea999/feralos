package config

import "testing"

func TestApplyImpliedFlagsEdgeRepos(t *testing.T) {
	tests := []struct {
		name        string
		edgePkgs    []string
		edgeEnabled bool
		wantEnabled bool
	}{
		{"edge packages imply repos", []string{"mesa@edge"}, false, true},
		{"already enabled stays", []string{"mesa@edge"}, true, true},
		{"no edge packages no flag", nil, false, false},
		{"no edge packages flag untouched", nil, true, true},
		{"empty list no flag", []string{}, false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conf := &Config{}
			conf.User.ExtraPackagesEdge = tt.edgePkgs
			conf.User.EnableEdgeRepos = tt.edgeEnabled
			applyImpliedFlags(conf)
			if conf.User.EnableEdgeRepos != tt.wantEnabled {
				t.Errorf("EnableEdgeRepos = %v, want %v", conf.User.EnableEdgeRepos, tt.wantEnabled)
			}
		})
	}
}

func TestValidateConfigEdgePackagesNoRepoFlagRequired(t *testing.T) {
	// extraPackagesEdge without enableEdgeRepos must NOT be a validation
	// error anymore — applyImpliedFlags derives the flag (LoadConfig path).
	conf := &Config{}
	conf.User.ExtraPackagesEdge = []string{"mesa@edge", "mesa-dri-gallium@edge"}
	conf.User.EnableEdgeRepos = false // simulates pre-implied state
	if err := validateConfig(conf); err != nil {
		t.Errorf("validateConfig with edge packages and no flag returned error: %v", err)
	}
}
