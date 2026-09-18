package main

import _ "embed"

// xlsxLibJS embeds the SheetJS "xlsx" community-edition core build (Apache-2.0,
// see the file's own header comment for attribution). It is injected as part of
// the page init script so the in-app document preview modal can parse real
// spreadsheet files -- both modern .xlsx (OOXML) and legacy binary .xls (BIFF/OLE2)
// -- entirely offline, without depending on any runtime network fetch.
//
//go:embed assets/xlsx.core.min.js
var xlsxLibJS string
