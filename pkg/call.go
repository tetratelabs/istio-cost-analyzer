package pkg

type Call struct {
	From     Locality
	To       Locality
	CallSize uint64
}

type Locality struct {
	Region  string
	Zone    string
	Subzone string
}
