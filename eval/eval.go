package eval

/* 
 repl provides a single function, Eval, that "evaluates" its argument. See documentation for Eval for more details

 author: Sriram Srinivasan (sriram@malhar.net)
*/

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

type strmap map[string]string

var builtinPkgs = map[string]string{
	"adler32":   "hash/adler32",
	"aes":       "crypto/aes",
	"ascii85":   "encoding/ascii85",
	"asn1":      "encoding/asn1",
	"ast":       "go/ast",
	"atomic":    "sync/atomic",
	"base32":    "encoding/base32",
	"base64":    "encoding/base64",
	"big":       "math/big",
	"binary":    "encoding/binary",
	"bufio":     "bufio",
	"build":     "go/build",
	"bytes":     "bytes",
	"bzip2":     "compress/bzip2",
	"cgi":       "net/http/cgi",
	"cgo":       "runtime/cgo",
	"cipher":    "crypto/cipher",
	"cmplx":     "math/cmplx",
	"color":     "image/color",
	"crc32":     "hash/crc32",
	"crc64":     "hash/crc64",
	"crypto":    "crypto",
	"csv":       "encoding/csv",
	"debug":     "runtime/debug",
	"des":       "crypto/des",
	"doc":       "go/doc",
	"draw":      "image/draw",
	"driver":    "database/sql/driver",
	"dsa":       "crypto/dsa",
	"dwarf":     "debug/dwarf",
	"ecdsa":     "crypto/ecdsa",
	"elf":       "debug/elf",
	"elliptic":  "crypto/elliptic",
	"errors":    "errors",
	"exec":      "os/exec",
	"expvar":    "expvar",
	"fcgi":      "net/http/fcgi",
	"filepath":  "path/filepath",
	"flag":      "flag",
	"flate":     "compress/flate",
	"fmt":       "fmt",
	"fnv":       "hash/fnv",
	"gif":       "image/gif",
	"gob":       "encoding/gob",
	"gosym":     "debug/gosym",
	"gzip":      "compress/gzip",
	"hash":      "hash",
	"heap":      "container/heap",
	"hex":       "encoding/hex",
	"hmac":      "crypto/hmac",
	"html":      "html",
	"http":      "net/http",
	"httputil":  "net/http/httputil",
	"image":     "image",
	"io":        "io",
	"ioutil":    "io/ioutil",
	"jpeg":      "image/jpeg",
	"json":      "encoding/json",
	"jsonrpc":   "net/rpc/jsonrpc",
	"list":      "container/list",
	"log":       "log",
	"lzw":       "compress/lzw",
	"macho":     "debug/macho",
	"mail":      "net/mail",
	"math":      "math",
	"md5":       "crypto/md5",
	"mime":      "mime",
	"multipart": "mime/multipart",
	"net":       "net",
	"os":        "os",
	"parse":     "text/template/parse",
	"parser":    "go/parser",
	"path":      "path",
	"pe":        "debug/pe",
	"pem":       "encoding/pem",
	"pkix":      "crypto/x509/pkix",
	"png":       "image/png",
	"pprof":     "net/http/pprof",
	//"pprof": "runtime/pprof",
	"printer": "go/printer",
	//"rand": "crypto/rand",
	"rand":    "math/rand",
	"rc4":     "crypto/rc4",
	"reflect": "reflect",
	"regexp":  "regexp",
	"ring":    "container/ring",
	"rpc":     "net/rpc",
	"rsa":     "crypto/rsa",
	"runtime": "runtime",
	//"scanner": "go/scanner",
	"scanner":     "text/scanner",
	"sha1":        "crypto/sha1",
	"sha256":      "crypto/sha256",
	"sha512":      "crypto/sha512",
	"signal":      "os/signal",
	"smtp":        "net/smtp",
	"sort":        "sort",
	"sql":         "database/sql",
	"strconv":     "strconv",
	"strings":     "strings",
	"subtle":      "crypto/subtle",
	"suffixarray": "index/suffixarray",
	"sync":        "sync",
	"syntax":      "regexp/syntax",
	"syscall":     "syscall",
	"syslog":      "log/syslog",
	"tabwriter":   "text/tabwriter",
	"tar":         "archive/tar",
	//"template": "html/template",
	//"template": "text/template",
	"textproto": "net/textproto",
	"time":      "time",
	"tls":       "crypto/tls",
	"token":     "go/token",
	"unicode":   "unicode",
	"unsafe":    "unsafe",
	"url":       "net/url",
	"user":      "os/user",
	"utf16":     "unicode/utf16",
	"utf8":      "unicode/utf8",
	"x509":      "crypto/x509",
	"xml":       "encoding/xml",
	"zip":       "archive/zip",
	"zlib":      "compress/zlib",
}

// Eval "evaluates" a multi-line bit of go code by compiling and running it. It
// returns either a non-blank compiler error, or the combined stdout and stderr output
// generated by the evaluated code.  
// Eval is designed to help interactive exploreation, and so provides
// the conveniences illustrated in the example below
//   Eval(`
//         p "Eval demo"
//         type A struct {
//               S string
//               V int
//         }
//         a := A{S: "The answer is", V: 42}
//         p "a = ", a
//         fmt.Printf("%s: %d\n", a.S, a.V)
// `)
// This should return: 
//     Eval demo
//     a =  {The answer is 42}
//     The answer is: 42
// 
// 1. A line of the form "p XXX" is translated to println(XXX). 
// 2. There is no need to import standard go packages. They are inferred
//    and imported automatically. (e.g. "fmt" in the code above)
// 3. The code is wrapped inside a main package and a main function. 
//    Explicit import statements, type declarations and func declarations
//    remain global (outside the main function)
// 

func Eval(code string) (out string, err string) {
	defer func() { // error recovery
		if e := recover(); e != nil {
			out = ""
			err = fmt.Sprintf("1:%v", e)
		}
	}()
	// No additional wrapping if it has a package declaration already
	if ok, _ := regexp.MatchString("^ *package ", code); ok {
		out, err = run(code)
		return out, err
	}

	code = expandAliases(code)
	pkgsToImport := inferPackages(code)
	code = embedLineNumbers(code)
	global, nonGlobal := partition(code)
	return buildAndExec(global, nonGlobal, pkgsToImport)
}

func expandAliases(code string) string {
	// Expand "p foo(), 2*3"   to println(foo(), 2*3)
	r := regexp.MustCompile(`(?m)^\s*p +(.*)$`)
	return string(r.ReplaceAll([]byte(code), []byte("__p($1)")))
}

// Each line of the original source is tagged with a line number at the end like so: //#100
// Since the wrapping process adds import statements and rearranges global and non-global 
// statements (see partition), this embedding permits us to map compiler error numbers back
// to the original source
func embedLineNumbers(code string) string {
	lineNum := 0
	if code[len(code)-1] != '\n' {
		code += "\n"
	}
	r := regexp.MustCompile("\n")
	return r.ReplaceAllStringFunc(code,
		func(string) string {
			lineNum++
			return fmt.Sprintf("//#%d\n", lineNum)
		})
}

// split code into global and non-global chunks. non-global chunks belong inside
// a main function, and global chunks refer to type, func and import declarations
func partition(code string) (global string, nonGlobal string) {
	r := regexp.MustCompile("^ *(func|type|import)")
	pos := 0 // Always maintained as the position from where to restart search
	for {
		chunk := nextChunk(code[pos:])
		//fmt.Println("CHUNK<<<" + chunk + ">>>")
		if len(chunk) == 0 {
			break
		}
		if r.FindString(chunk) == "" { // not import, type or func decl. 
			nonGlobal += chunk
		} else {
			global += chunk
		}
		pos += len(chunk)
	}
	return
}

var pkgPattern = regexp.MustCompile(`[a-z]\w+\.`)

func inferPackages(chunk string) (pkgsToImport map[string]bool) {
	pkgsToImport = make(map[string]bool) // used as a set
	pkgs := pkgPattern.FindAllString(chunk, 100000)
	for _, pkg := range pkgs {
		pkg = pkg[:len(pkg)-1] // remove trailing '.'
		if importPkg, ok := builtinPkgs[pkg]; ok {
			pkgsToImport[importPkg] = true
		}
	}
	return pkgsToImport
}

func buildAndExec(global string, nonGlobal string, pkgsToImport map[string]bool) (out string, err string) {
	src := buildMain(global, nonGlobal, pkgsToImport)
	out, err = run(src)
	if err != "" {
		if repairImports(err, pkgsToImport) {
			src = buildMain(global, nonGlobal, pkgsToImport)
			out, err = run(src)
		}
	}
	return out, err
}

func repairImports(err string, pkgsToImport map[string]bool) (dupsDetected bool) {
	// Look for compile errors of the form
	// "test.go:10: xxx redeclared as imported package name"
	// and remove 'xxx' from pkgsToImport
	dupsDetected = false
	var pkg string
	r := regexp.MustCompile(`(?m)(\w+) redeclared as imported package name|imported and not used: "(\w+)"`)
	for _, match := range r.FindAllStringSubmatch(err, -1) {
		// Either $1 or $2 will have name of pkg name that's been imported
		if match[1] != "" {
			pkg = match[1]
		} else if match[2] != "" {
			pkg = match[2]
		}
		if pkgsToImport[pkg] {
			// Was the duplicate import our mistake, due to an incorrect guess? If so ... 
			delete(pkgsToImport, pkg)
			dupsDetected = true
		}
	}
	return dupsDetected
}

func run(src string) (output string, err string) {
	src, newToOldLineNums := extractLineNumbers(src)
	tmpfile := save(src)
	cmd := exec.Command("go", "run", tmpfile)
	out, e := cmd.CombinedOutput()

	if e != nil {
		err = string(out)
		return "", remapCompileErrorLines(err, newToOldLineNums)
	} else {
		return string(out), ""
	}
	return "", ""
}

func remapCompileErrorLines(err string, newToOldLineNums map[int]int) string {
	ret := ""
	r := regexp.MustCompile(`^.*?:(\d+):`)
	for _, line := range strings.Split(err, "\n") {
		if len(line) == 0 || strings.HasPrefix(line, "# command-line-arguments") {
			continue
		}
		if m := r.FindStringSubmatchIndex(line); m != nil {
			newLine, err := strconv.Atoi(line[m[2]:m[3]]) // The $1 slice
			if err != nil {
				panic("Internal error: Unable to convert " + line[m[2]:m[3]])
			}
			oldLine := newToOldLineNums[newLine]
			ret += fmt.Sprintf("%d:%s\n", oldLine, line[(m[3]+1):])
		} else {
			ret += line + "\n"
		}
	}
	return ret
}

func extractLineNumbers(src string) (srcNoLineNums string, newToOldLineNums map[int]int) {
	newToOldLineNums = make(map[int]int)
	r := regexp.MustCompile(`(?m)//#(\d+)$`)
	for newLineNum, line := range strings.Split(src, "\n") {
		if m := r.FindStringSubmatch(line); m != nil {
			oldLineNum, _ := strconv.Atoi(m[1])
			newToOldLineNums[newLineNum+1] = oldLineNum // compiler errors are 1-based
		}
	}
	srcNoLineNums = r.ReplaceAllString(src, "") // remove line number annotations
	return
}

func save(src string) (tmpfile string) {
	tmpfile = tempDir() + string(os.PathSeparator) + "gore_eval.go"
	fh, err := os.OpenFile(tmpfile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0666)
	if err != nil {
		panic("Unable to open file: '" + tmpfile + "': " + err.Error())
	}
	fh.WriteString(src)
	fh.Close()
	return tmpfile
}

func buildMain(global string, nonGlobal string, pkgsToImport map[string]bool) string {
	imports := ""
	delete(pkgsToImport, "fmt") // Explicitly importing fmt in main
	for k, _ := range pkgsToImport {
		imports += `import "` + k + "\"\n"
	}
	template := `
package main
import "fmt"
%s
func __p(values ...interface{}){
	for _, v := range values {
             fmt.Printf(%s, v)
	}
}
%s
func main() {
     %s
}
`
	valuefmt := `"%v\n"` // Embedding %v into template expands it prematurely!
	return fmt.Sprintf(template, imports, valuefmt, global, nonGlobal)
}

var openParenPattern = regexp.MustCompile(`(\{|\() *//#\d+$`)
var nlPattern = regexp.MustCompile(` *//#\d+\n`)
// if line ends with '{' or '(', then consume until the corresponding '}' or ')'. Else return the next line.
func nextChunk(code string) (chunk string) {
	// get earliest of '{', '(' or '\n'
	var ch, closech rune
	var i int

	i = strings.Index(code, "\n")
	pos := i + 1
	if i == 0 {
		return code[:pos]
	} // first char is newline
	if i == -1 {
		return code
	} // EOS

	// Does it end with '{' or '('?  Note, line numbers have been embedded, so we look for the form '{ //#234\n'
	parenloc := openParenPattern.FindStringIndex(code[:i])
	if parenloc == nil {
		return code[:pos]
	}
	switch ch = rune(code[parenloc[0]]); ch {
	case '{':
		closech = '}'
	case '(':
		closech = ')'
	default:
		return code[:i]
	}

	// Search for closing ch, allowing for nesting. Note: '{' and '(' embedded within strings are incorrectly counted
	startch := ch
	count := 1
	for i, ch = range code[pos:] {
		if ch == startch {
			count++
		} else if ch == closech {
			count--
			if count == 0 {
				break
			}
		}
	}
	pos += i + 1
	if count != 0 {
		panic(fmt.Sprintf("Mismatched parentheses or brackets:%s", code[:pos]))
	}
	// consume trailing spaces and newline, plus embedded line number pattern, if any
	nlloc := nlPattern.FindStringIndex(code[pos:])
	if nlloc != nil {
		pos += nlloc[1]
	}

	return code[:pos]
}

func tempDir() string {
	dir := os.Getenv("TMPDIR")
	if dir == "" {
		dir = "/tmp"
	}
	return dir
}