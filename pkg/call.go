package pkg

import "fmt"

type Call struct {
	From         *Locality
	FromWorkload string
	To           *Locality
	ToWorkload   string
	CallSize     uint64
}

func (c *Call) String() string {
	return fmt.Sprintf("%v (%v)->%v (%v) : %v", c.FromWorkload, c.From.Zone, c.ToWorkload, c.To.Zone, c.CallSize)
}

type PodCall struct {
	FromPod      string
	FromWorkload string
	ToPod        string
	ToWorkload   string
	CallSize     uint64
}

type Locality struct {
	Region  string
	Zone    string
	Subzone string
}
