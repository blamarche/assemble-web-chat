package assemble

// Err general error holder
type Err struct {
	m string
}

func (e *Err) Error() string {
	return e.m
}
