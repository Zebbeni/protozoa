package environment

// LookupAPI provides functions to look up items and organisms
type LookupAPI interface {
	GetPh() float64
}

type ChangeAPI interface {
	UpdatePh(change float64)
}

// API provides functions needed to lookup and make changes to environment objects
type API interface {
	LookupAPI
	ChangeAPI
}
