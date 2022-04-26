package environment

// API provides functions to look up information about the sim state
type API interface {
	Cycle() int
}
