package main

// Microcode Assembler

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"reflect"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"

	"github.com/Univ-Wyo-Education/S20-2150/Mac"
	"github.com/pschlump/MiscLib"
	"github.com/pschlump/filelib"
	"github.com/pschlump/godebug"
	"gitlab.com/pschlump/PureImaginationServer/ymux"
)

// xyzzy421 - Add in --version
// xyzzy401 - ImplementDebugFlags

// ---------------------------------------------------------------------------------
// asm - MARIA assembler.
// ---------------------------------------------------------------------------------
// --in  FILE.mas	input .mas file
// --out FILE.hex	output assembled code
// --st  file.out   Symbol table output
// ---------------------------------------------------------------------------------

var In = flag.String("in", "", "Input File - microcode assembly code. (microcode.mm)")
var Out = flag.String("out", "", "Output in hex. Loadable Microcode .hex file")
var DbFlag = flag.String("db-flag", "", "debug flags.") // xyzzy401 - TODO
var St = flag.String("st", "", "Output symbol table to file")

var stOut = os.Stdout

var OnWindows = false

func init() {
	if runtime.GOOS == "windows" {
		OnWindows = true
	}
	OnWindows = true
}

func main() {

	flag.Parse() // Parse CLI arguments

	fns := flag.Args()

	if len(fns) > 0 {
		fmt.Fprintf(os.Stderr, "Invalid arguments\n")
		os.Exit(1)
	}

	// xyzzy401 - ImplementDebugFlags
	if *In == "" {
		fmt.Printf("Fatal: Required command line parameter --in FILE.mas is missing\n")
		os.Exit(1)
	}
	if *Out == "" {
		fmt.Printf("Fatal: Required command line parameter --out FILE.hex is missing\n")
		os.Exit(1)
	}

	fn := *In
	out := *Out

	if *St != "" {
		var err error
		stOut, err = filelib.Fopen(*St, "w")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Erorr oping symbol table output %s : error : %s\n", *St, err)
			os.Exit(1)
		}
	}

	// process lines in file...
	buf, err := ioutil.ReadFile(fn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to open %s - assembly language input file:%s\n", fn, err)
		os.Exit(1)
	}
	mes := string(buf)
	mes_lines := strings.Split(mes, "\n")

	n_err := 0

	if db14 {
	}

	memBuf0 := make([]uint64, 256, 256) // Memory is 256 address, 64 wide

	mpc := 0
	for ii, line := range mes_lines {
		line_no := ii + 1

		line = strings.TrimRight(line, "\r\n")
		line = removeComment(line)
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			continue
		}

		// Type of Parsed Lines

		symbols, op_s, err := ParseLine(line, line_no)
		_, _, _ = symbols, op_s, err
		if symbols == nil || len(symbols) == 0 {
			continue
		}

		if op_s == "__end__" {
			break
		}
		if op_s == "DCL" {
			for _, ss := range symbols[1:] {
				AddSymbol(ss, line_no, true)
			}
			continue
		}
		if op_s == "ORG" {
			if len(symbols) >= 1 {
				mpc = convertAddr(symbols[1], line_no)
			} else {
				fmt.Printf("Missing address for ORG, Line %d\n", line_no)
			}
			continue
		}

		for _, ss := range symbols {
			AddSymbol(ss, line_no, false)
		}

		eu := uint64(0)

		for _, ss := range symbols {
			st, err := LookupSymbol(ss)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Invalid symbol [%s] was not found, line %d\n", ss, line_no)
			} else {
				eu = eu | (1 << st.Address)
			}

		}

		memBuf0[mpc&0xff] = eu
		mpc++

	}

	if db1 {
		DumpSymbolTable(stOut)
	}

	// Output
	if n_err > 0 {
		fmt.Fprintf(os.Stderr, "%s# Of Errors: %d%s\n", MiscLib.ColorRed, n_err, MiscLib.ColorReset)
		fmt.Fprintf(os.Stderr, ".hex file may be incorrect\n")
	}
	outFp, err := filelib.Fopen(out, "w")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to open output file : %s error : %s\n", out, err)
		os.Exit(1)
	}
	for ii, aaa := range memBuf0 {
		fmt.Fprintf(outFp, "%016x %03d\n", aaa, ii)
	}
	outFp.Close()
	if n_err > 0 {
		os.Exit(3)
	}
}

func convertAddr(h string, line_no int) int {
	if len(h) > 2 && h[0:2] == "0b" {
		h = h[2:]
		h = strings.Replace(h, "_", "", -1)
		rv, err := strconv.ParseInt(h, 2, 64)
		if err != nil {
			fmt.Printf("invalid binary number [%s], line no:%d\n", h, line_no)
		}
		return int(rv)
	} else {
		fmt.Printf("Invalid - ORG should be followd by a 0x000000000 address, line_no:%d\n", line_no)
	}
	return 0
}

//////////////////////////////////////////////////////////////////////////////////////////////////////////////////
// Parsing
//////////////////////////////////////////////////////////////////////////////////////////////////////////////////
func ParseLine(line string, line_no int) (symbols []string, op_s string, err error) {

	symbols = []string{}

	// ORG <value>
	// Symbol  Symbol Symbol
	//
	// #define Name Value
	// Symbol = ID
	r := regexp.MustCompile("[^\\s]+")
	symbols = r.FindAllString(line, -1)

	if len(symbols) > 0 && strings.ToLower(symbols[0]) == "org" {
		op_s = "ORG"
	} else if len(symbols) > 0 && strings.ToLower(symbols[0]) == "dcl" {
		op_s = "DCL"
	} else if len(symbols) > 0 && strings.ToLower(symbols[0]) == "__end__" {
		op_s = "__end__"
	} else {
		op_s = "1"
	}
	fmt.Printf("symbols ->%s<- op %s\n", godebug.SVar(symbols), op_s)

	return
}

//////////////////////////////////////////////////////////////////////////////////////////////////////////////////
// Symbol table
//////////////////////////////////////////////////////////////////////////////////////////////////////////////////

type SymbolTableType struct {
	Name     string
	LineNo   []int
	Address  int
	Declared bool
}

var SymbolTable map[string]SymbolTableType
var SymbolAddress int

func init() {
	SymbolTable = make(map[string]SymbolTableType)
	SymbolAddress = 0
}

func AddSymbol(Name string, line_no int, Dcl bool) (err error) {
	if ss, found := SymbolTable[Name]; !found {
		if !Dcl {
			fmt.Fprintf(os.Stderr, "%sFond non-declared symbol (%s) on line %d%s\n", MiscLib.ColorRed, Name, line_no, MiscLib.ColorReset)
		}
		SymbolTable[Name] = SymbolTableType{
			Name:     Name,
			LineNo:   []int{line_no},
			Address:  SymbolAddress,
			Declared: Dcl,
		}
		SymbolAddress++
	} else {
		ss.LineNo = append(ss.LineNo, line_no)
		SymbolTable[Name] = ss
	}
	return
}

func LookupSymbol(Name string) (st SymbolTableType, err error) {
	var ok bool
	st, ok = SymbolTable[Name]
	if !ok {
		err = fmt.Errorf("Not Found")
	}
	return
}

// KeysFromMap returns an array of keys from a map.
//
// This is used like this:
//
//	keys := KeysFromMap(nameMap)
//	sort.Strings(keys)
//	for _, key := range keys {
//		val := nameMap[key]
//		...
//	}
//
func KeysFromMap(a interface{}) (keys []string) {
	xkeys := reflect.ValueOf(a).MapKeys()
	keys = make([]string, len(xkeys))
	for ii, vv := range xkeys {
		keys[ii] = vv.String()
	}
	return
}

func DumpSymbolTable(fp *os.File) {
	fmt.Fprintf(fp, "Symbol Table\n")
	fmt.Fprintf(fp, "-------------------------------------------------------------\n")
	keys := ymux.KeysFromMap(SymbolTable)
	sort.Strings(keys)
	// for key, val := range SymbolTable {
	for _, key := range keys {
		val := SymbolTable[key]
		fmt.Fprintf(fp, "%s: %s\n", key, godebug.SVar(val))
	}
	fmt.Fprintf(fp, "-------------------------------------------------------------\n\n")
}

//////////////////////////////////////////////////////////////////////////////////////////////////////////////////
// Utitlieis
//////////////////////////////////////////////////////////////////////////////////////////////////////////////////

func MaxAddress(a, b Mac.AddressType) Mac.AddressType {
	if a > b {
		return a
	}
	return b
}

func removeComment(line string) (rv string) {
	rv = line
	for i := range line {
		if line[i] == '/' {
			return line[0:i]
		}
	}
	return
}

var db1 = true  // Leave True
var db2 = false // Debug of Parsing code		// xyzzy
var db8 = false
var db7 = false
var db5 = false  // HEX directive w/ hex output
var db10 = false // test STR directive
var db12 = false // test STR directive
var db14 = true  // DOS