// AUTO-GENERATED by mkbuiltin.go; DO NOT EDIT

package gc

const runtimeimport = "" +
	"package runtime safe\n" +
	"type @\"\".hbucket uint8\n" +
	"type @\"\".hmap struct { @\"\".count int; @\"\".flags uint8; B uint8; @\"\".hash0 uint32; @\"\".buckets *@\"\".hbucket; @\"\".oldbuckets *@\"\".hbucket; @\"\".nevacuate uintptr; @\"\".overflow *[2]*[]*@\"\".hbucket }\n" +
	"type @\"\".hiter struct { @\"\".key *byte; @\"\".value *byte; @\"\".t *byte; @\"\".h *@\"\".hmap; @\"\".buckets *@\"\".hbucket; @\"\".bptr *@\"\".hbucket; @\"\".overflow [2]*[]*@\"\".hbucket; @\"\".startBucket uintptr; @\"\".offset uint8; @\"\".wrapped bool; B uint8; @\"\".i uint8; @\"\".bucket uintptr; @\"\".checkBucket uintptr }\n" +
	"type @\"\".scase struct { @\"\".elem *byte; @\"\".c *byte; @\"\".pc uintptr; @\"\".kind uint16; @\"\".so uint16; @\"\".receivedp *bool; @\"\".releasetime int64 }\n" +
	"func @\"\".newobject (@\"\".typ·2 *byte) (? *any)\n" +
	"func @\"\".panicindex ()\n" +
	"func @\"\".panicslice ()\n" +
	"func @\"\".panicdivide ()\n" +
	"func @\"\".throwreturn ()\n" +
	"func @\"\".throwinit ()\n" +
	"func @\"\".panicwrap (? string, ? string, ? string)\n" +
	"func @\"\".gopanic (? interface {})\n" +
	"func @\"\".gorecover (? *int32) (? interface {})\n" +
	"func @\"\".printbool (? bool)\n" +
	"func @\"\".printfloat (? float64)\n" +
	"func @\"\".printint (? int64)\n" +
	"func @\"\".printhex (? uint64)\n" +
	"func @\"\".printuint (? uint64)\n" +
	"func @\"\".printcomplex (? complex128)\n" +
	"func @\"\".printstring (? string)\n" +
	"func @\"\".printpointer (? any)\n" +
	"func @\"\".printiface (? any)\n" +
	"func @\"\".printeface (? any)\n" +
	"func @\"\".printslice (? any)\n" +
	"func @\"\".printnl ()\n" +
	"func @\"\".printsp ()\n" +
	"func @\"\".printlock ()\n" +
	"func @\"\".printunlock ()\n" +
	"func @\"\".concatstring2 (? *[32]byte, ? string, ? string) (? string)\n" +
	"func @\"\".concatstring3 (? *[32]byte, ? string, ? string, ? string) (? string)\n" +
	"func @\"\".concatstring4 (? *[32]byte, ? string, ? string, ? string, ? string) (? string)\n" +
	"func @\"\".concatstring5 (? *[32]byte, ? string, ? string, ? string, ? string, ? string) (? string)\n" +
	"func @\"\".concatstrings (? *[32]byte, ? []string) (? string)\n" +
	"func @\"\".cmpstring (? string, ? string) (? int)\n" +
	"func @\"\".eqstring (? string, ? string) (? bool)\n" +
	"func @\"\".intstring (? *[4]byte, ? int64) (? string)\n" +
	"func @\"\".slicebytetostring (? *[32]byte, ? []byte) (? string)\n" +
	"func @\"\".slicebytetostringtmp (? []byte) (? string)\n" +
	"func @\"\".slicerunetostring (? *[32]byte, ? []rune) (? string)\n" +
	"func @\"\".stringtoslicebyte (? *[32]byte, ? string) (? []byte)\n" +
	"func @\"\".stringtoslicebytetmp (? string) (? []byte)\n" +
	"func @\"\".stringtoslicerune (? *[32]rune, ? string) (? []rune)\n" +
	"func @\"\".stringiter (? string, ? int) (? int)\n" +
	"func @\"\".stringiter2 (? string, ? int) (@\"\".retk·1 int, @\"\".retv·2 rune)\n" +
	"func @\"\".slicecopy (@\"\".to·2 any, @\"\".fr·3 any, @\"\".wid·4 uintptr \"unsafe-uintptr\") (? int)\n" +
	"func @\"\".slicestringcopy (@\"\".to·2 any, @\"\".fr·3 any) (? int)\n" +
	"func @\"\".typ2Itab (@\"\".typ·2 *byte, @\"\".typ2·3 *byte, @\"\".cache·4 **byte) (@\"\".ret·1 *byte)\n" +
	"func @\"\".convI2E (@\"\".elem·2 any) (@\"\".ret·1 any)\n" +
	"func @\"\".convI2I (@\"\".typ·2 *byte, @\"\".elem·3 any) (@\"\".ret·1 any)\n" +
	"func @\"\".convT2E (@\"\".typ·2 *byte, @\"\".elem·3 *any, @\"\".buf·4 *any) (@\"\".ret·1 any)\n" +
	"func @\"\".convT2I (@\"\".typ·2 *byte, @\"\".typ2·3 *byte, @\"\".cache·4 **byte, @\"\".elem·5 *any, @\"\".buf·6 *any) (@\"\".ret·1 any)\n" +
	"func @\"\".assertE2E (@\"\".typ·1 *byte, @\"\".iface·2 any, @\"\".ret·3 *any)\n" +
	"func @\"\".assertE2E2 (@\"\".typ·2 *byte, @\"\".iface·3 any, @\"\".ret·4 *any) (? bool)\n" +
	"func @\"\".assertE2I (@\"\".typ·1 *byte, @\"\".iface·2 any, @\"\".ret·3 *any)\n" +
	"func @\"\".assertE2I2 (@\"\".typ·2 *byte, @\"\".iface·3 any, @\"\".ret·4 *any) (? bool)\n" +
	"func @\"\".assertE2T (@\"\".typ·1 *byte, @\"\".iface·2 any, @\"\".ret·3 *any)\n" +
	"func @\"\".assertE2T2 (@\"\".typ·2 *byte, @\"\".iface·3 any, @\"\".ret·4 *any) (? bool)\n" +
	"func @\"\".assertI2E (@\"\".typ·1 *byte, @\"\".iface·2 any, @\"\".ret·3 *any)\n" +
	"func @\"\".assertI2E2 (@\"\".typ·2 *byte, @\"\".iface·3 any, @\"\".ret·4 *any) (? bool)\n" +
	"func @\"\".assertI2I (@\"\".typ·1 *byte, @\"\".iface·2 any, @\"\".ret·3 *any)\n" +
	"func @\"\".assertI2I2 (@\"\".typ·2 *byte, @\"\".iface·3 any, @\"\".ret·4 *any) (? bool)\n" +
	"func @\"\".assertI2T (@\"\".typ·1 *byte, @\"\".iface·2 any, @\"\".ret·3 *any)\n" +
	"func @\"\".assertI2T2 (@\"\".typ·2 *byte, @\"\".iface·3 any, @\"\".ret·4 *any) (? bool)\n" +
	"func @\"\".panicdottype (@\"\".have·1 *byte, @\"\".want·2 *byte, @\"\".iface·3 *byte)\n" +
	"func @\"\".ifaceeq (@\"\".i1·2 any, @\"\".i2·3 any) (@\"\".ret·1 bool)\n" +
	"func @\"\".efaceeq (@\"\".i1·2 any, @\"\".i2·3 any) (@\"\".ret·1 bool)\n" +
	"func @\"\".makemap (@\"\".mapType·2 *byte, @\"\".hint·3 int64, @\"\".mapbuf·4 *@\"\".hmap, @\"\".bucketbuf·5 *any) (@\"\".hmap·1 map[any]any)\n" +
	"func @\"\".mapaccess1 (@\"\".mapType·2 *byte, @\"\".hmap·3 map[any]any, @\"\".key·4 *any) (@\"\".val·1 *any)\n" +
	"func @\"\".mapaccess1_fast32 (@\"\".mapType·2 *byte, @\"\".hmap·3 map[any]any, @\"\".key·4 any) (@\"\".val·1 *any)\n" +
	"func @\"\".mapaccess1_fast64 (@\"\".mapType·2 *byte, @\"\".hmap·3 map[any]any, @\"\".key·4 any) (@\"\".val·1 *any)\n" +
	"func @\"\".mapaccess1_faststr (@\"\".mapType·2 *byte, @\"\".hmap·3 map[any]any, @\"\".key·4 any) (@\"\".val·1 *any)\n" +
	"func @\"\".mapaccess2 (@\"\".mapType·3 *byte, @\"\".hmap·4 map[any]any, @\"\".key·5 *any) (@\"\".val·1 *any, @\"\".pres·2 bool)\n" +
	"func @\"\".mapaccess2_fast32 (@\"\".mapType·3 *byte, @\"\".hmap·4 map[any]any, @\"\".key·5 any) (@\"\".val·1 *any, @\"\".pres·2 bool)\n" +
	"func @\"\".mapaccess2_fast64 (@\"\".mapType·3 *byte, @\"\".hmap·4 map[any]any, @\"\".key·5 any) (@\"\".val·1 *any, @\"\".pres·2 bool)\n" +
	"func @\"\".mapaccess2_faststr (@\"\".mapType·3 *byte, @\"\".hmap·4 map[any]any, @\"\".key·5 any) (@\"\".val·1 *any, @\"\".pres·2 bool)\n" +
	"func @\"\".mapassign1 (@\"\".mapType·1 *byte, @\"\".hmap·2 map[any]any, @\"\".key·3 *any, @\"\".val·4 *any)\n" +
	"func @\"\".mapiterinit (@\"\".mapType·1 *byte, @\"\".hmap·2 map[any]any, @\"\".hiter·3 *@\"\".hiter)\n" +
	"func @\"\".mapdelete (@\"\".mapType·1 *byte, @\"\".hmap·2 map[any]any, @\"\".key·3 *any)\n" +
	"func @\"\".mapiternext (@\"\".hiter·1 *@\"\".hiter)\n" +
	"func @\"\".makechan (@\"\".chanType·2 *byte, @\"\".hint·3 int64) (@\"\".hchan·1 chan any)\n" +
	"func @\"\".chanrecv1 (@\"\".chanType·1 *byte, @\"\".hchan·2 <-chan any, @\"\".elem·3 *any)\n" +
	"func @\"\".chanrecv2 (@\"\".chanType·2 *byte, @\"\".hchan·3 <-chan any, @\"\".elem·4 *any) (? bool)\n" +
	"func @\"\".chansend1 (@\"\".chanType·1 *byte, @\"\".hchan·2 chan<- any, @\"\".elem·3 *any)\n" +
	"func @\"\".closechan (@\"\".hchan·1 any)\n" +
	"var @\"\".writeBarrier struct { @\"\".enabled bool; @\"\".needed bool; @\"\".cgo bool }\n" +
	"func @\"\".writebarrierptr (@\"\".dst·1 *any, @\"\".src·2 any)\n" +
	"func @\"\".writebarrierstring (@\"\".dst·1 *any, @\"\".src·2 any)\n" +
	"func @\"\".writebarrierslice (@\"\".dst·1 *any, @\"\".src·2 any)\n" +
	"func @\"\".writebarrieriface (@\"\".dst·1 *any, @\"\".src·2 any)\n" +
	"func @\"\".writebarrierfat01 (@\"\".dst·1 *any, _ uintptr \"unsafe-uintptr\", @\"\".src·3 any)\n" +
	"func @\"\".writebarrierfat10 (@\"\".dst·1 *any, _ uintptr \"unsafe-uintptr\", @\"\".src·3 any)\n" +
	"func @\"\".writebarrierfat11 (@\"\".dst·1 *any, _ uintptr \"unsafe-uintptr\", @\"\".src·3 any)\n" +
	"func @\"\".writebarrierfat001 (@\"\".dst·1 *any, _ uintptr \"unsafe-uintptr\", @\"\".src·3 any)\n" +
	"func @\"\".writebarrierfat010 (@\"\".dst·1 *any, _ uintptr \"unsafe-uintptr\", @\"\".src·3 any)\n" +
	"func @\"\".writebarrierfat011 (@\"\".dst·1 *any, _ uintptr \"unsafe-uintptr\", @\"\".src·3 any)\n" +
	"func @\"\".writebarrierfat100 (@\"\".dst·1 *any, _ uintptr \"unsafe-uintptr\", @\"\".src·3 any)\n" +
	"func @\"\".writebarrierfat101 (@\"\".dst·1 *any, _ uintptr \"unsafe-uintptr\", @\"\".src·3 any)\n" +
	"func @\"\".writebarrierfat110 (@\"\".dst·1 *any, _ uintptr \"unsafe-uintptr\", @\"\".src·3 any)\n" +
	"func @\"\".writebarrierfat111 (@\"\".dst·1 *any, _ uintptr \"unsafe-uintptr\", @\"\".src·3 any)\n" +
	"func @\"\".writebarrierfat0001 (@\"\".dst·1 *any, _ uintptr \"unsafe-uintptr\", @\"\".src·3 any)\n" +
	"func @\"\".writebarrierfat0010 (@\"\".dst·1 *any, _ uintptr \"unsafe-uintptr\", @\"\".src·3 any)\n" +
	"func @\"\".writebarrierfat0011 (@\"\".dst·1 *any, _ uintptr \"unsafe-uintptr\", @\"\".src·3 any)\n" +
	"func @\"\".writebarrierfat0100 (@\"\".dst·1 *any, _ uintptr \"unsafe-uintptr\", @\"\".src·3 any)\n" +
	"func @\"\".writebarrierfat0101 (@\"\".dst·1 *any, _ uintptr \"unsafe-uintptr\", @\"\".src·3 any)\n" +
	"func @\"\".writebarrierfat0110 (@\"\".dst·1 *any, _ uintptr \"unsafe-uintptr\", @\"\".src·3 any)\n" +
	"func @\"\".writebarrierfat0111 (@\"\".dst·1 *any, _ uintptr \"unsafe-uintptr\", @\"\".src·3 any)\n" +
	"func @\"\".writebarrierfat1000 (@\"\".dst·1 *any, _ uintptr \"unsafe-uintptr\", @\"\".src·3 any)\n" +
	"func @\"\".writebarrierfat1001 (@\"\".dst·1 *any, _ uintptr \"unsafe-uintptr\", @\"\".src·3 any)\n" +
	"func @\"\".writebarrierfat1010 (@\"\".dst·1 *any, _ uintptr \"unsafe-uintptr\", @\"\".src·3 any)\n" +
	"func @\"\".writebarrierfat1011 (@\"\".dst·1 *any, _ uintptr \"unsafe-uintptr\", @\"\".src·3 any)\n" +
	"func @\"\".writebarrierfat1100 (@\"\".dst·1 *any, _ uintptr \"unsafe-uintptr\", @\"\".src·3 any)\n" +
	"func @\"\".writebarrierfat1101 (@\"\".dst·1 *any, _ uintptr \"unsafe-uintptr\", @\"\".src·3 any)\n" +
	"func @\"\".writebarrierfat1110 (@\"\".dst·1 *any, _ uintptr \"unsafe-uintptr\", @\"\".src·3 any)\n" +
	"func @\"\".writebarrierfat1111 (@\"\".dst·1 *any, _ uintptr \"unsafe-uintptr\", @\"\".src·3 any)\n" +
	"func @\"\".typedmemmove (@\"\".typ·1 *byte, @\"\".dst·2 *any, @\"\".src·3 *any)\n" +
	"func @\"\".typedslicecopy (@\"\".typ·2 *byte, @\"\".dst·3 any, @\"\".src·4 any) (? int)\n" +
	"func @\"\".selectnbsend (@\"\".chanType·2 *byte, @\"\".hchan·3 chan<- any, @\"\".elem·4 *any) (? bool)\n" +
	"func @\"\".selectnbrecv (@\"\".chanType·2 *byte, @\"\".elem·3 *any, @\"\".hchan·4 <-chan any) (? bool)\n" +
	"func @\"\".selectnbrecv2 (@\"\".chanType·2 *byte, @\"\".elem·3 *any, @\"\".received·4 *bool, @\"\".hchan·5 <-chan any) (? bool)\n" +
	"func @\"\".newselect (@\"\".sel·1 *byte, @\"\".selsize·2 int64, @\"\".size·3 int32)\n" +
	"func @\"\".selectsend (@\"\".sel·2 *byte, @\"\".hchan·3 chan<- any, @\"\".elem·4 *any) (@\"\".selected·1 bool)\n" +
	"func @\"\".selectrecv (@\"\".sel·2 *byte, @\"\".hchan·3 <-chan any, @\"\".elem·4 *any) (@\"\".selected·1 bool)\n" +
	"func @\"\".selectrecv2 (@\"\".sel·2 *byte, @\"\".hchan·3 <-chan any, @\"\".elem·4 *any, @\"\".received·5 *bool) (@\"\".selected·1 bool)\n" +
	"func @\"\".selectdefault (@\"\".sel·2 *byte) (@\"\".selected·1 bool)\n" +
	"func @\"\".selectgo (@\"\".sel·1 *byte)\n" +
	"func @\"\".block ()\n" +
	"func @\"\".makeslice (@\"\".typ·2 *byte, @\"\".nel·3 int64, @\"\".cap·4 int64) (@\"\".ary·1 []any)\n" +
	"func @\"\".growslice (@\"\".typ·2 *byte, @\"\".old·3 []any, @\"\".cap·4 int) (@\"\".ary·1 []any)\n" +
	"func @\"\".growslice_n (@\"\".typ·2 *byte, @\"\".old·3 []any, @\"\".n·4 int) (@\"\".ary·1 []any)\n" +
	"func @\"\".memmove (@\"\".to·1 *any, @\"\".frm·2 *any, @\"\".length·3 uintptr \"unsafe-uintptr\")\n" +
	"func @\"\".memclr (@\"\".ptr·1 *byte, @\"\".length·2 uintptr \"unsafe-uintptr\")\n" +
	"func @\"\".memequal (@\"\".x·2 *any, @\"\".y·3 *any, @\"\".size·4 uintptr \"unsafe-uintptr\") (? bool)\n" +
	"func @\"\".memequal8 (@\"\".x·2 *any, @\"\".y·3 *any) (? bool)\n" +
	"func @\"\".memequal16 (@\"\".x·2 *any, @\"\".y·3 *any) (? bool)\n" +
	"func @\"\".memequal32 (@\"\".x·2 *any, @\"\".y·3 *any) (? bool)\n" +
	"func @\"\".memequal64 (@\"\".x·2 *any, @\"\".y·3 *any) (? bool)\n" +
	"func @\"\".memequal128 (@\"\".x·2 *any, @\"\".y·3 *any) (? bool)\n" +
	"func @\"\".int64div (? int64, ? int64) (? int64)\n" +
	"func @\"\".uint64div (? uint64, ? uint64) (? uint64)\n" +
	"func @\"\".int64mod (? int64, ? int64) (? int64)\n" +
	"func @\"\".uint64mod (? uint64, ? uint64) (? uint64)\n" +
	"func @\"\".float64toint64 (? float64) (? int64)\n" +
	"func @\"\".float64touint64 (? float64) (? uint64)\n" +
	"func @\"\".int64tofloat64 (? int64) (? float64)\n" +
	"func @\"\".uint64tofloat64 (? uint64) (? float64)\n" +
	"func @\"\".complex128div (@\"\".num·2 complex128, @\"\".den·3 complex128) (@\"\".quo·1 complex128)\n" +
	"func @\"\".racefuncenter (? uintptr \"unsafe-uintptr\")\n" +
	"func @\"\".racefuncexit ()\n" +
	"func @\"\".raceread (? uintptr \"unsafe-uintptr\")\n" +
	"func @\"\".racewrite (? uintptr \"unsafe-uintptr\")\n" +
	"func @\"\".racereadrange (@\"\".addr·1 uintptr \"unsafe-uintptr\", @\"\".size·2 uintptr \"unsafe-uintptr\")\n" +
	"func @\"\".racewriterange (@\"\".addr·1 uintptr \"unsafe-uintptr\", @\"\".size·2 uintptr \"unsafe-uintptr\")\n" +
	"func @\"\".msanread (@\"\".addr·1 uintptr \"unsafe-uintptr\", @\"\".size·2 uintptr \"unsafe-uintptr\")\n" +
	"func @\"\".msanwrite (@\"\".addr·1 uintptr \"unsafe-uintptr\", @\"\".size·2 uintptr \"unsafe-uintptr\")\n" +
	"\n" +
	"$$\n"

const unsafeimport = "" +
	"package unsafe\n" +
	"type @\"\".Pointer uintptr\n" +
	"func @\"\".Offsetof (? any) (? uintptr)\n" +
	"func @\"\".Sizeof (? any) (? uintptr)\n" +
	"func @\"\".Alignof (? any) (? uintptr)\n" +
	"\n" +
	"$$\n"
