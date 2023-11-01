package rbac

import "testing"

func TestRBACManager_IsEnabled(t *testing.T) {
	tests := []struct {
		name string
		m    *RBACManager
		want bool
	}{
		{
			name: "rbac enabled",
			m:    &RBACManager{enabled: true},
			want: true,
		},
		{
			name: "rbac disabled",
			m:    &RBACManager{enabled: false},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.m.IsEnabled()
			if got != tt.want {
				t.Errorf("RBACManager.IsEnabled() = %v, want %v", got, tt.want)
			}
		})
	}
}
