//go:generate sh -c "printf %s $(git describe --always --long 2> /dev/null || echo '0.0.0+devel') > version.txt"

package main

import _ "embed"

//go:embed version.txt
var Version string
