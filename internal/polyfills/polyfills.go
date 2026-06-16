package polyfills

import _ "embed"

//go:embed dom.js
var DomScript string

//go:embed xhr.js
var XhrScript string

//go:embed cookies.js
var CookiesScript string

//go:embed fetch.js
var FetchScript string
