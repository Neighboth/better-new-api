//go:build !windows

package main

// The window of the client of the installation.
//
// The systems that are not Windows do not carry the runtime of WebView2, the
// page of the client is opened with the browser of the system.

import "fmt"

// runWebViewHelper is not used on the systems that are not Windows.
func runWebViewHelper(url string, title string, iconPath string) int {
	fmt.Println("The window of the client is not available on this system, the page is opened with the browser.")
	return 2
}
