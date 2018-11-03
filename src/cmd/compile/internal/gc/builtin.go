// Code generated by mkbuiltin.go. DO NOT EDIT.

package gc

import "cmd/compile/internal/types"

var runtimeDecls = [...]struct {
	name string
	tag  int
	typ  int
}{
	{"newobject", funcTag, 4},
	{"panicindex", funcTag, 5},
	{"panicslice", funcTag, 5},
	{"panicdivide", funcTag, 5},
	{"panicmakeslicelen", funcTag, 5},
	{"throwinit", funcTag, 5},
	{"panicwrap", funcTag, 5},
	{"gopanic", funcTag, 7},
	{"gorecover", funcTag, 10},
	{"goschedguarded", funcTag, 5},
	{"printbool", funcTag, 12},
	{"printfloat", funcTag, 14},
	{"printint", funcTag, 16},
	{"printhex", funcTag, 18},
	{"printuint", funcTag, 18},
	{"printcomplex", funcTag, 20},
	{"printstring", funcTag, 22},
	{"printpointer", funcTag, 23},
	{"printiface", funcTag, 23},
	{"printeface", funcTag, 23},
	{"printslice", funcTag, 23},
	{"printnl", funcTag, 5},
	{"printsp", funcTag, 5},
	{"printlock", funcTag, 5},
	{"printunlock", funcTag, 5},
	{"concatstring2", funcTag, 26},
	{"concatstring3", funcTag, 27},
	{"concatstring4", funcTag, 28},
	{"concatstring5", funcTag, 29},
	{"concatstrings", funcTag, 31},
	{"cmpstring", funcTag, 33},
	{"intstring", funcTag, 36},
	{"slicebytetostring", funcTag, 38},
	{"slicebytetostringtmp", funcTag, 39},
	{"slicerunetostring", funcTag, 42},
	{"stringtoslicebyte", funcTag, 43},
	{"stringtoslicerune", funcTag, 46},
	{"slicecopy", funcTag, 48},
	{"slicestringcopy", funcTag, 49},
	{"decoderune", funcTag, 50},
	{"countrunes", funcTag, 51},
	{"convI2I", funcTag, 52},
	{"convT2E", funcTag, 53},
	{"convT2E16", funcTag, 52},
	{"convT2E32", funcTag, 52},
	{"convT2E64", funcTag, 52},
	{"convT2Estring", funcTag, 52},
	{"convT2Eslice", funcTag, 52},
	{"convT2Enoptr", funcTag, 53},
	{"convT2I", funcTag, 53},
	{"convT2I16", funcTag, 52},
	{"convT2I32", funcTag, 52},
	{"convT2I64", funcTag, 52},
	{"convT2Istring", funcTag, 52},
	{"convT2Islice", funcTag, 52},
	{"convT2Inoptr", funcTag, 53},
	{"assertE2I", funcTag, 52},
	{"assertE2I2", funcTag, 54},
	{"assertI2I", funcTag, 52},
	{"assertI2I2", funcTag, 54},
	{"panicdottypeE", funcTag, 55},
	{"panicdottypeI", funcTag, 55},
	{"panicnildottype", funcTag, 56},
	{"ifaceeq", funcTag, 59},
	{"efaceeq", funcTag, 59},
	{"fastrand", funcTag, 61},
	{"makemap64", funcTag, 63},
	{"makemap", funcTag, 64},
	{"makemap_small", funcTag, 65},
	{"mapaccess1", funcTag, 66},
	{"mapaccess1_fast32", funcTag, 67},
	{"mapaccess1_fast64", funcTag, 67},
	{"mapaccess1_faststr", funcTag, 67},
	{"mapaccess1_fat", funcTag, 68},
	{"mapaccess2", funcTag, 69},
	{"mapaccess2_fast32", funcTag, 70},
	{"mapaccess2_fast64", funcTag, 70},
	{"mapaccess2_faststr", funcTag, 70},
	{"mapaccess2_fat", funcTag, 71},
	{"mapassign", funcTag, 66},
	{"mapassign_fast32", funcTag, 67},
	{"mapassign_fast32ptr", funcTag, 67},
	{"mapassign_fast64", funcTag, 67},
	{"mapassign_fast64ptr", funcTag, 67},
	{"mapassign_faststr", funcTag, 67},
	{"mapiterinit", funcTag, 72},
	{"mapdelete", funcTag, 72},
	{"mapdelete_fast32", funcTag, 73},
	{"mapdelete_fast64", funcTag, 73},
	{"mapdelete_faststr", funcTag, 73},
	{"mapiternext", funcTag, 74},
	{"mapclear", funcTag, 75},
	{"makechan64", funcTag, 77},
	{"makechan", funcTag, 78},
	{"chanrecv1", funcTag, 80},
	{"chanrecv2", funcTag, 81},
	{"chansend1", funcTag, 83},
	{"closechan", funcTag, 23},
	{"writeBarrier", varTag, 85},
	{"typedmemmove", funcTag, 86},
	{"typedmemclr", funcTag, 87},
	{"typedslicecopy", funcTag, 88},
	{"selectnbsend", funcTag, 89},
	{"selectnbrecv", funcTag, 90},
	{"selectnbrecv2", funcTag, 92},
	{"selectsetpc", funcTag, 56},
	{"selectgo", funcTag, 93},
	{"block", funcTag, 5},
	{"makeslice", funcTag, 94},
	{"makeslice64", funcTag, 95},
	{"growslice", funcTag, 97},
	{"memmove", funcTag, 98},
	{"memclrNoHeapPointers", funcTag, 99},
	{"memclrHasPointers", funcTag, 99},
	{"memequal", funcTag, 100},
	{"memequal8", funcTag, 101},
	{"memequal16", funcTag, 101},
	{"memequal32", funcTag, 101},
	{"memequal64", funcTag, 101},
	{"memequal128", funcTag, 101},
	{"int64div", funcTag, 102},
	{"uint64div", funcTag, 103},
	{"int64mod", funcTag, 102},
	{"uint64mod", funcTag, 103},
	{"float64toint64", funcTag, 104},
	{"float64touint64", funcTag, 105},
	{"float64touint32", funcTag, 106},
	{"int64tofloat64", funcTag, 107},
	{"uint64tofloat64", funcTag, 108},
	{"uint32tofloat64", funcTag, 109},
	{"complex128div", funcTag, 110},
	{"racefuncenter", funcTag, 111},
	{"racefuncenterfp", funcTag, 5},
	{"racefuncexit", funcTag, 5},
	{"raceread", funcTag, 111},
	{"racewrite", funcTag, 111},
	{"racereadrange", funcTag, 112},
	{"racewriterange", funcTag, 112},
	{"msanread", funcTag, 112},
	{"msanwrite", funcTag, 112},
	{"support_popcnt", varTag, 11},
	{"support_sse41", varTag, 11},
}

func runtimeTypes() []*types.Type {
	var typs [113]*types.Type
	typs[0] = types.Bytetype
	typs[1] = types.NewPtr(typs[0])
	typs[2] = types.Types[TANY]
	typs[3] = types.NewPtr(typs[2])
	typs[4] = functype(nil, []*Node{anonfield(typs[1])}, []*Node{anonfield(typs[3])})
	typs[5] = functype(nil, nil, nil)
	typs[6] = types.Types[TINTER]
	typs[7] = functype(nil, []*Node{anonfield(typs[6])}, nil)
	typs[8] = types.Types[TINT32]
	typs[9] = types.NewPtr(typs[8])
	typs[10] = functype(nil, []*Node{anonfield(typs[9])}, []*Node{anonfield(typs[6])})
	typs[11] = types.Types[TBOOL]
	typs[12] = functype(nil, []*Node{anonfield(typs[11])}, nil)
	typs[13] = types.Types[TFLOAT64]
	typs[14] = functype(nil, []*Node{anonfield(typs[13])}, nil)
	typs[15] = types.Types[TINT64]
	typs[16] = functype(nil, []*Node{anonfield(typs[15])}, nil)
	typs[17] = types.Types[TUINT64]
	typs[18] = functype(nil, []*Node{anonfield(typs[17])}, nil)
	typs[19] = types.Types[TCOMPLEX128]
	typs[20] = functype(nil, []*Node{anonfield(typs[19])}, nil)
	typs[21] = types.Types[TSTRING]
	typs[22] = functype(nil, []*Node{anonfield(typs[21])}, nil)
	typs[23] = functype(nil, []*Node{anonfield(typs[2])}, nil)
	typs[24] = types.NewArray(typs[0], 32)
	typs[25] = types.NewPtr(typs[24])
	typs[26] = functype(nil, []*Node{anonfield(typs[25]), anonfield(typs[21]), anonfield(typs[21])}, []*Node{anonfield(typs[21])})
	typs[27] = functype(nil, []*Node{anonfield(typs[25]), anonfield(typs[21]), anonfield(typs[21]), anonfield(typs[21])}, []*Node{anonfield(typs[21])})
	typs[28] = functype(nil, []*Node{anonfield(typs[25]), anonfield(typs[21]), anonfield(typs[21]), anonfield(typs[21]), anonfield(typs[21])}, []*Node{anonfield(typs[21])})
	typs[29] = functype(nil, []*Node{anonfield(typs[25]), anonfield(typs[21]), anonfield(typs[21]), anonfield(typs[21]), anonfield(typs[21]), anonfield(typs[21])}, []*Node{anonfield(typs[21])})
	typs[30] = types.NewSlice(typs[21])
	typs[31] = functype(nil, []*Node{anonfield(typs[25]), anonfield(typs[30])}, []*Node{anonfield(typs[21])})
	typs[32] = types.Types[TINT]
	typs[33] = functype(nil, []*Node{anonfield(typs[21]), anonfield(typs[21])}, []*Node{anonfield(typs[32])})
	typs[34] = types.NewArray(typs[0], 4)
	typs[35] = types.NewPtr(typs[34])
	typs[36] = functype(nil, []*Node{anonfield(typs[35]), anonfield(typs[15])}, []*Node{anonfield(typs[21])})
	typs[37] = types.NewSlice(typs[0])
	typs[38] = functype(nil, []*Node{anonfield(typs[25]), anonfield(typs[37])}, []*Node{anonfield(typs[21])})
	typs[39] = functype(nil, []*Node{anonfield(typs[37])}, []*Node{anonfield(typs[21])})
	typs[40] = types.Runetype
	typs[41] = types.NewSlice(typs[40])
	typs[42] = functype(nil, []*Node{anonfield(typs[25]), anonfield(typs[41])}, []*Node{anonfield(typs[21])})
	typs[43] = functype(nil, []*Node{anonfield(typs[25]), anonfield(typs[21])}, []*Node{anonfield(typs[37])})
	typs[44] = types.NewArray(typs[40], 32)
	typs[45] = types.NewPtr(typs[44])
	typs[46] = functype(nil, []*Node{anonfield(typs[45]), anonfield(typs[21])}, []*Node{anonfield(typs[41])})
	typs[47] = types.Types[TUINTPTR]
	typs[48] = functype(nil, []*Node{anonfield(typs[2]), anonfield(typs[2]), anonfield(typs[47])}, []*Node{anonfield(typs[32])})
	typs[49] = functype(nil, []*Node{anonfield(typs[2]), anonfield(typs[2])}, []*Node{anonfield(typs[32])})
	typs[50] = functype(nil, []*Node{anonfield(typs[21]), anonfield(typs[32])}, []*Node{anonfield(typs[40]), anonfield(typs[32])})
	typs[51] = functype(nil, []*Node{anonfield(typs[21])}, []*Node{anonfield(typs[32])})
	typs[52] = functype(nil, []*Node{anonfield(typs[1]), anonfield(typs[2])}, []*Node{anonfield(typs[2])})
	typs[53] = functype(nil, []*Node{anonfield(typs[1]), anonfield(typs[3])}, []*Node{anonfield(typs[2])})
	typs[54] = functype(nil, []*Node{anonfield(typs[1]), anonfield(typs[2])}, []*Node{anonfield(typs[2]), anonfield(typs[11])})
	typs[55] = functype(nil, []*Node{anonfield(typs[1]), anonfield(typs[1]), anonfield(typs[1])}, nil)
	typs[56] = functype(nil, []*Node{anonfield(typs[1])}, nil)
	typs[57] = types.NewPtr(typs[47])
	typs[58] = types.Types[TUNSAFEPTR]
	typs[59] = functype(nil, []*Node{anonfield(typs[57]), anonfield(typs[58]), anonfield(typs[58])}, []*Node{anonfield(typs[11])})
	typs[60] = types.Types[TUINT32]
	typs[61] = functype(nil, nil, []*Node{anonfield(typs[60])})
	typs[62] = types.NewMap(typs[2], typs[2])
	typs[63] = functype(nil, []*Node{anonfield(typs[1]), anonfield(typs[15]), anonfield(typs[3])}, []*Node{anonfield(typs[62])})
	typs[64] = functype(nil, []*Node{anonfield(typs[1]), anonfield(typs[32]), anonfield(typs[3])}, []*Node{anonfield(typs[62])})
	typs[65] = functype(nil, nil, []*Node{anonfield(typs[62])})
	typs[66] = functype(nil, []*Node{anonfield(typs[1]), anonfield(typs[62]), anonfield(typs[3])}, []*Node{anonfield(typs[3])})
	typs[67] = functype(nil, []*Node{anonfield(typs[1]), anonfield(typs[62]), anonfield(typs[2])}, []*Node{anonfield(typs[3])})
	typs[68] = functype(nil, []*Node{anonfield(typs[1]), anonfield(typs[62]), anonfield(typs[3]), anonfield(typs[1])}, []*Node{anonfield(typs[3])})
	typs[69] = functype(nil, []*Node{anonfield(typs[1]), anonfield(typs[62]), anonfield(typs[3])}, []*Node{anonfield(typs[3]), anonfield(typs[11])})
	typs[70] = functype(nil, []*Node{anonfield(typs[1]), anonfield(typs[62]), anonfield(typs[2])}, []*Node{anonfield(typs[3]), anonfield(typs[11])})
	typs[71] = functype(nil, []*Node{anonfield(typs[1]), anonfield(typs[62]), anonfield(typs[3]), anonfield(typs[1])}, []*Node{anonfield(typs[3]), anonfield(typs[11])})
	typs[72] = functype(nil, []*Node{anonfield(typs[1]), anonfield(typs[62]), anonfield(typs[3])}, nil)
	typs[73] = functype(nil, []*Node{anonfield(typs[1]), anonfield(typs[62]), anonfield(typs[2])}, nil)
	typs[74] = functype(nil, []*Node{anonfield(typs[3])}, nil)
	typs[75] = functype(nil, []*Node{anonfield(typs[1]), anonfield(typs[62])}, nil)
	typs[76] = types.NewChan(typs[2], types.Cboth)
	typs[77] = functype(nil, []*Node{anonfield(typs[1]), anonfield(typs[15])}, []*Node{anonfield(typs[76])})
	typs[78] = functype(nil, []*Node{anonfield(typs[1]), anonfield(typs[32])}, []*Node{anonfield(typs[76])})
	typs[79] = types.NewChan(typs[2], types.Crecv)
	typs[80] = functype(nil, []*Node{anonfield(typs[79]), anonfield(typs[3])}, nil)
	typs[81] = functype(nil, []*Node{anonfield(typs[79]), anonfield(typs[3])}, []*Node{anonfield(typs[11])})
	typs[82] = types.NewChan(typs[2], types.Csend)
	typs[83] = functype(nil, []*Node{anonfield(typs[82]), anonfield(typs[3])}, nil)
	typs[84] = types.NewArray(typs[0], 3)
	typs[85] = tostruct([]*Node{namedfield("enabled", typs[11]), namedfield("pad", typs[84]), namedfield("needed", typs[11]), namedfield("cgo", typs[11]), namedfield("alignme", typs[17])})
	typs[86] = functype(nil, []*Node{anonfield(typs[1]), anonfield(typs[3]), anonfield(typs[3])}, nil)
	typs[87] = functype(nil, []*Node{anonfield(typs[1]), anonfield(typs[3])}, nil)
	typs[88] = functype(nil, []*Node{anonfield(typs[1]), anonfield(typs[2]), anonfield(typs[2])}, []*Node{anonfield(typs[32])})
	typs[89] = functype(nil, []*Node{anonfield(typs[82]), anonfield(typs[3])}, []*Node{anonfield(typs[11])})
	typs[90] = functype(nil, []*Node{anonfield(typs[3]), anonfield(typs[79])}, []*Node{anonfield(typs[11])})
	typs[91] = types.NewPtr(typs[11])
	typs[92] = functype(nil, []*Node{anonfield(typs[3]), anonfield(typs[91]), anonfield(typs[79])}, []*Node{anonfield(typs[11])})
	typs[93] = functype(nil, []*Node{anonfield(typs[1]), anonfield(typs[1]), anonfield(typs[32])}, []*Node{anonfield(typs[32]), anonfield(typs[11])})
	typs[94] = functype(nil, []*Node{anonfield(typs[1]), anonfield(typs[32]), anonfield(typs[32])}, []*Node{anonfield(typs[58])})
	typs[95] = functype(nil, []*Node{anonfield(typs[1]), anonfield(typs[15]), anonfield(typs[15])}, []*Node{anonfield(typs[58])})
	typs[96] = types.NewSlice(typs[2])
	typs[97] = functype(nil, []*Node{anonfield(typs[1]), anonfield(typs[96]), anonfield(typs[32])}, []*Node{anonfield(typs[96])})
	typs[98] = functype(nil, []*Node{anonfield(typs[3]), anonfield(typs[3]), anonfield(typs[47])}, nil)
	typs[99] = functype(nil, []*Node{anonfield(typs[58]), anonfield(typs[47])}, nil)
	typs[100] = functype(nil, []*Node{anonfield(typs[3]), anonfield(typs[3]), anonfield(typs[47])}, []*Node{anonfield(typs[11])})
	typs[101] = functype(nil, []*Node{anonfield(typs[3]), anonfield(typs[3])}, []*Node{anonfield(typs[11])})
	typs[102] = functype(nil, []*Node{anonfield(typs[15]), anonfield(typs[15])}, []*Node{anonfield(typs[15])})
	typs[103] = functype(nil, []*Node{anonfield(typs[17]), anonfield(typs[17])}, []*Node{anonfield(typs[17])})
	typs[104] = functype(nil, []*Node{anonfield(typs[13])}, []*Node{anonfield(typs[15])})
	typs[105] = functype(nil, []*Node{anonfield(typs[13])}, []*Node{anonfield(typs[17])})
	typs[106] = functype(nil, []*Node{anonfield(typs[13])}, []*Node{anonfield(typs[60])})
	typs[107] = functype(nil, []*Node{anonfield(typs[15])}, []*Node{anonfield(typs[13])})
	typs[108] = functype(nil, []*Node{anonfield(typs[17])}, []*Node{anonfield(typs[13])})
	typs[109] = functype(nil, []*Node{anonfield(typs[60])}, []*Node{anonfield(typs[13])})
	typs[110] = functype(nil, []*Node{anonfield(typs[19]), anonfield(typs[19])}, []*Node{anonfield(typs[19])})
	typs[111] = functype(nil, []*Node{anonfield(typs[47])}, nil)
	typs[112] = functype(nil, []*Node{anonfield(typs[47]), anonfield(typs[47])}, nil)
	return typs[:]
}
