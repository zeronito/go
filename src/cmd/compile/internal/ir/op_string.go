// Code generated by "stringer -type=Op -trimprefix=O node.go"; DO NOT EDIT.

package ir

import "strconv"

func _() {
	// An "invalid array index" compiler error signifies that the constant values have changed.
	// Re-run the stringer command to generate them again.
	var x [1]struct{}
	_ = x[OXXX-0]
	_ = x[ONAME-1]
	_ = x[ONONAME-2]
	_ = x[OTYPE-3]
	_ = x[OLITERAL-4]
	_ = x[ONIL-5]
	_ = x[OADD-6]
	_ = x[OSUB-7]
	_ = x[OOR-8]
	_ = x[OXOR-9]
	_ = x[OADDSTR-10]
	_ = x[OADDR-11]
	_ = x[OANDAND-12]
	_ = x[OAPPEND-13]
	_ = x[OBYTES2STR-14]
	_ = x[OBYTES2STRTMP-15]
	_ = x[ORUNES2STR-16]
	_ = x[OSTR2BYTES-17]
	_ = x[OSTR2BYTESTMP-18]
	_ = x[OSTR2RUNES-19]
	_ = x[OSLICE2ARR-20]
	_ = x[OSLICE2ARRPTR-21]
	_ = x[OAS-22]
	_ = x[OAS2-23]
	_ = x[OAS2DOTTYPE-24]
	_ = x[OAS2FUNC-25]
	_ = x[OAS2MAPR-26]
	_ = x[OAS2RECV-27]
	_ = x[OASOP-28]
	_ = x[OCALL-29]
	_ = x[OCALLFUNC-30]
	_ = x[OCALLMETH-31]
	_ = x[OCALLINTER-32]
	_ = x[OCAP-33]
	_ = x[OCLEAR-34]
	_ = x[OCLOSE-35]
	_ = x[OCLOSURE-36]
	_ = x[OCOMPLIT-37]
	_ = x[OMAPLIT-38]
	_ = x[OSTRUCTLIT-39]
	_ = x[OARRAYLIT-40]
	_ = x[OSLICELIT-41]
	_ = x[OPTRLIT-42]
	_ = x[OCONV-43]
	_ = x[OCONVIFACE-44]
	_ = x[OCONVNOP-45]
	_ = x[OCOPY-46]
	_ = x[ODCL-47]
	_ = x[ODCLGOLOCAL-48]
	_ = x[ODCLGOLOCALALLOC-49]
	_ = x[ODCLFUNC-50]
	_ = x[ODELETE-51]
	_ = x[ODOT-52]
	_ = x[ODOTPTR-53]
	_ = x[ODOTMETH-54]
	_ = x[ODOTINTER-55]
	_ = x[OXDOT-56]
	_ = x[ODOTTYPE-57]
	_ = x[ODOTTYPE2-58]
	_ = x[OEQ-59]
	_ = x[ONE-60]
	_ = x[OLT-61]
	_ = x[OLE-62]
	_ = x[OGE-63]
	_ = x[OGT-64]
	_ = x[ODEREF-65]
	_ = x[OINDEX-66]
	_ = x[OINDEXMAP-67]
	_ = x[OKEY-68]
	_ = x[OSTRUCTKEY-69]
	_ = x[OLEN-70]
	_ = x[OMAKE-71]
	_ = x[OMAKECHAN-72]
	_ = x[OMAKEMAP-73]
	_ = x[OMAKESLICE-74]
	_ = x[OMAKESLICECOPY-75]
	_ = x[OMUL-76]
	_ = x[ODIV-77]
	_ = x[OMOD-78]
	_ = x[OLSH-79]
	_ = x[ORSH-80]
	_ = x[OAND-81]
	_ = x[OANDNOT-82]
	_ = x[ONEW-83]
	_ = x[ONOT-84]
	_ = x[OBITNOT-85]
	_ = x[OPLUS-86]
	_ = x[ONEG-87]
	_ = x[OOROR-88]
	_ = x[OPANIC-89]
	_ = x[OPRINT-90]
	_ = x[OPRINTLN-91]
	_ = x[OPAREN-92]
	_ = x[OSEND-93]
	_ = x[OSLICE-94]
	_ = x[OSLICEARR-95]
	_ = x[OSLICESTR-96]
	_ = x[OSLICE3-97]
	_ = x[OSLICE3ARR-98]
	_ = x[OSLICEHEADER-99]
	_ = x[OSTRINGHEADER-100]
	_ = x[ORECOVER-101]
	_ = x[ORECOVERFP-102]
	_ = x[ORECV-103]
	_ = x[ORUNESTR-104]
	_ = x[OSELRECV2-105]
	_ = x[OMIN-106]
	_ = x[OMAX-107]
	_ = x[OREAL-108]
	_ = x[OIMAG-109]
	_ = x[OCOMPLEX-110]
	_ = x[OUNSAFEADD-111]
	_ = x[OUNSAFESLICE-112]
	_ = x[OUNSAFESLICEDATA-113]
	_ = x[OUNSAFESTRING-114]
	_ = x[OUNSAFESTRINGDATA-115]
	_ = x[OMETHEXPR-116]
	_ = x[OMETHVALUE-117]
	_ = x[OBLOCK-118]
	_ = x[OBREAK-119]
	_ = x[OCASE-120]
	_ = x[OCONTINUE-121]
	_ = x[ODEFER-122]
	_ = x[OFALL-123]
	_ = x[OFOR-124]
	_ = x[OGOTO-125]
	_ = x[OIF-126]
	_ = x[OLABEL-127]
	_ = x[OGO-128]
	_ = x[ORANGE-129]
	_ = x[ORETURN-130]
	_ = x[OSELECT-131]
	_ = x[OSWITCH-132]
	_ = x[OTYPESW-133]
	_ = x[OINLCALL-134]
	_ = x[OMAKEFACE-135]
	_ = x[OITAB-136]
	_ = x[OIDATA-137]
	_ = x[OSPTR-138]
	_ = x[OCFUNC-139]
	_ = x[OCHECKNIL-140]
	_ = x[ORESULT-141]
	_ = x[OINLMARK-142]
	_ = x[OLINKSYMOFFSET-143]
	_ = x[OJUMPTABLE-144]
	_ = x[OINTERFACESWITCH-145]
	_ = x[ODYNAMICDOTTYPE-146]
	_ = x[ODYNAMICDOTTYPE2-147]
	_ = x[ODYNAMICTYPE-148]
	_ = x[OTAILCALL-149]
	_ = x[OGETG-150]
	_ = x[OGETCALLERPC-151]
	_ = x[OGETCALLERSP-152]
	_ = x[OEND-153]
}

const _Op_name = "XXXNAMENONAMETYPELITERALNILADDSUBORXORADDSTRADDRANDANDAPPENDBYTES2STRBYTES2STRTMPRUNES2STRSTR2BYTESSTR2BYTESTMPSTR2RUNESSLICE2ARRSLICE2ARRPTRASAS2AS2DOTTYPEAS2FUNCAS2MAPRAS2RECVASOPCALLCALLFUNCCALLMETHCALLINTERCAPCLEARCLOSECLOSURECOMPLITMAPLITSTRUCTLITARRAYLITSLICELITPTRLITCONVCONVIFACECONVNOPCOPYDCLDCLGOLOCALDCLGOLOCALALLOCDCLFUNCDELETEDOTDOTPTRDOTMETHDOTINTERXDOTDOTTYPEDOTTYPE2EQNELTLEGEGTDEREFINDEXINDEXMAPKEYSTRUCTKEYLENMAKEMAKECHANMAKEMAPMAKESLICEMAKESLICECOPYMULDIVMODLSHRSHANDANDNOTNEWNOTBITNOTPLUSNEGORORPANICPRINTPRINTLNPARENSENDSLICESLICEARRSLICESTRSLICE3SLICE3ARRSLICEHEADERSTRINGHEADERRECOVERRECOVERFPRECVRUNESTRSELRECV2MINMAXREALIMAGCOMPLEXUNSAFEADDUNSAFESLICEUNSAFESLICEDATAUNSAFESTRINGUNSAFESTRINGDATAMETHEXPRMETHVALUEBLOCKBREAKCASECONTINUEDEFERFALLFORGOTOIFLABELGORANGERETURNSELECTSWITCHTYPESWINLCALLMAKEFACEITABIDATASPTRCFUNCCHECKNILRESULTINLMARKLINKSYMOFFSETJUMPTABLEINTERFACESWITCHDYNAMICDOTTYPEDYNAMICDOTTYPE2DYNAMICTYPETAILCALLGETGGETCALLERPCGETCALLERSPEND"

var _Op_index = [...]uint16{0, 3, 7, 13, 17, 24, 27, 30, 33, 35, 38, 44, 48, 54, 60, 69, 81, 90, 99, 111, 120, 129, 141, 143, 146, 156, 163, 170, 177, 181, 185, 193, 201, 210, 213, 218, 223, 230, 237, 243, 252, 260, 268, 274, 278, 287, 294, 298, 301, 311, 326, 333, 339, 342, 348, 355, 363, 367, 374, 382, 384, 386, 388, 390, 392, 394, 399, 404, 412, 415, 424, 427, 431, 439, 446, 455, 468, 471, 474, 477, 480, 483, 486, 492, 495, 498, 504, 508, 511, 515, 520, 525, 532, 537, 541, 546, 554, 562, 568, 577, 588, 600, 607, 616, 620, 627, 635, 638, 641, 645, 649, 656, 665, 676, 691, 703, 719, 727, 736, 741, 746, 750, 758, 763, 767, 770, 774, 776, 781, 783, 788, 794, 800, 806, 812, 819, 827, 831, 836, 840, 845, 853, 859, 866, 879, 888, 903, 917, 932, 943, 951, 955, 966, 977, 980}

func (i Op) String() string {
	if i >= Op(len(_Op_index)-1) {
		return "Op(" + strconv.FormatInt(int64(i), 10) + ")"
	}
	return _Op_name[_Op_index[i]:_Op_index[i+1]]
}
