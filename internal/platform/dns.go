package platform

import (
	"fmt"
)

type DNSStatus struct {
	Active bool
	Status string
}

func (s *DNSStatus) String() string {
	icon := "🔓"
	if !s.Active {
		icon = "⚠️"
	}
	return fmt.Sprintf("%s %s", icon, s.Status)
}
