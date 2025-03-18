package cargo

import "regexp"

func clearOrigins() {
	crateRequestOrigins = []*regexp.Regexp{}
}

func SetTestOrigins(origins []*regexp.Regexp) func() {
	crateRequestOrigins = append([]*regexp.Regexp{}, origins...)

	return clearOrigins
}
