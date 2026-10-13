// Package v1 is the Montage version 1 engine API.
//
// Version matches the montage field in a project and the renderPlan field
// in an exported command list. A later Montage is a separate package,
// kbvmaker/v2. This package keeps accepting documents whose montage
// field is 1.
package v1

import "kbvmaker/v1/version"

// Version is the Montage document version this API implements.
const Version = version.Current
