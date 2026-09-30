package branding

import _ "embed"

// Icon is shared by the Windows executable and notification-area icon.
//
//go:embed javboss.ico
var Icon []byte
