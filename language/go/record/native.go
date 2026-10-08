// Purpose: the record's view of native code: which files of a package are
// not Go source, and every program the C toolchain used to build them,
// each by hash.
// Never:   names a tool Go did not name, or a file Go did not list.
package record

// NativeCode is a package whose build compiles or links files that are not
// Go source, listed by the kinds `go list` reports.
type NativeCode struct {
	Package         string   `json:"package"`
	CgoFiles        []string `json:"cgo_files"`
	CFiles          []string `json:"c_files"`
	CxxFiles        []string `json:"cxx_files"`
	ObjectiveCFiles []string `json:"objective_c_files"`
	HeaderFiles     []string `json:"header_files"`
	FortranFiles    []string `json:"fortran_files"`
	AssemblyFiles   []string `json:"assembly_files"`
	SwigFiles       []string `json:"swig_files"`
	SwigCxxFiles    []string `json:"swig_cxx_files"`
	ObjectFiles     []string `json:"object_files"`
}

// Tool is one program the C toolchain uses, by hash, with its version line
// when it prints one.
type Tool struct {
	File    FileDigest `json:"file"`
	Version string     `json:"version,omitempty"`
}

// CToolchain is every program cgo builds with, when cgo is on: the C
// compiler and SDK, the other compilers and helpers Go names, and the
// linker the C compiler reports. A tool Go names but the machine lacks
// is nil.
type CToolchain struct {
	Compiler   FileDigest `json:"compiler"`
	Version    string     `json:"version"`
	SdkPath    *string    `json:"sdk_path"`
	SdkVersion *string    `json:"sdk_version"`
	Cxx        *Tool      `json:"cxx"`
	Fortran    *Tool      `json:"fortran"`
	Archiver   *Tool      `json:"archiver"`
	PkgConfig  *Tool      `json:"pkg_config"`
	Linker     *Tool      `json:"linker"`
}
