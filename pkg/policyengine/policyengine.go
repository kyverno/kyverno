package policyengine

// As the logic to process the policies in stateless, we do not need to define struct and implement behaviors for it
// Instead we expose them as standalone functions passing the logger and the required atrributes
// The each function returns the changes that need to be applied on the resource
// the caller is responsible to apply the changes to the resource
