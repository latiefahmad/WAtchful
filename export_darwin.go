//go:build darwin

package main

/*
#include <stdlib.h>
*/
import "C"

// WAtchfulSwitchProfile is exported to Objective-C and relaunches the app
// into the chosen profile when it is picked from the tray/menubar Profiles
// submenu (see the extern declaration in app_darwin.go's cgo preamble).
//
// It lives in its own file ON PURPOSE: cgo compiles the preamble of every
// file containing a //export directive twice (once for the package's cgo
// objects, once for _cgo_export.c). app_darwin.go defines Objective-C
// classes in its preamble, which would then be linked twice (duplicate
// OBJC_CLASS_/OBJC_METACLASS_ symbols). This file's preamble is trivial.
//
//export WAtchfulSwitchProfile
func WAtchfulSwitchProfile(name *C.char) {
	switchToProfileByName(C.GoString(name))
}
