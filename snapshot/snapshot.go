package snapshot

import (
	"fmt"
	"time"
)

type Snapshot struct {
	Name    string
	UUID    string
	Created time.Time
}

func (s Snapshot) String() string {
	return fmt.Sprintf("%s (%s)", s.Name, s.UUID)
}
