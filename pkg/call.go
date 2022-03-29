package pkg

import "fmt"

type Call struct {
	From     *Locality
	To       *Locality
	CallSize uint64
}

func (c *Call) String() string {
	return fmt.Sprintf("%v->%v (%v)\n", c.From.Zone, c.To.Zone, c.CallSize)
}

type PodCall struct {
	FromPod  string
	ToPod    string
	CallSize uint64
}

type Locality struct {
	Region  string
	Zone    string
	Subzone string
}
