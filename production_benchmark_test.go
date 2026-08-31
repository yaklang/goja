package goja

import (
	"strconv"
	"strings"
	"testing"

	"github.com/yaklang/goja/parser"
)

func makeProductionBundle(targetBytes int) string {
	const prefix = `(()=>{const modules={`
	const suffix = `};return Object.keys(modules).length})()`
	const bodyPrefix = `:(module,exports)=>{const matcher=/(?<=contact )[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}/;exports.run=(input)=>matcher.test(input);exports.meta={enabled:true,retries:3,tags:["prod","min"]}},`

	var source strings.Builder
	source.Grow(targetBytes)
	source.WriteString(prefix)
	for index := 0; source.Len()+len(bodyPrefix)+len(suffix)+16 < targetBytes; index++ {
		source.WriteString(strconv.Itoa(index))
		source.WriteString(bodyPrefix)
	}
	source.WriteString(suffix)
	return source.String()
}

func BenchmarkCompileProductionBundle(b *testing.B) {
	for _, size := range []int{5 << 20, 20 << 20} {
		source := makeProductionBundle(size)
		b.Run(strconv.Itoa(size>>20)+"MiB", func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(len(source)))
			for i := 0; i < b.N; i++ {
				if _, err := Compile("production.min.js", source, false); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkParseProductionBundle(b *testing.B) {
	for _, size := range []int{5 << 20, 20 << 20} {
		source := makeProductionBundle(size)
		b.Run(strconv.Itoa(size>>20)+"MiB", func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(len(source)))
			for i := 0; i < b.N; i++ {
				if _, err := parser.ParseFile(nil, "production.min.js", source, 0); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkExecuteProductionBundle(b *testing.B) {
	for _, size := range []int{5 << 20, 20 << 20} {
		source := makeProductionBundle(size)
		program, err := Compile("production.min.js", source, false)
		if err != nil {
			b.Fatal(err)
		}
		b.Run(strconv.Itoa(size>>20)+"MiB", func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(len(source)))
			for i := 0; i < b.N; i++ {
				if _, err := New().RunProgram(program); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkLoadProductionBundle(b *testing.B) {
	for _, size := range []int{5 << 20, 20 << 20} {
		source := makeProductionBundle(size)
		b.Run(strconv.Itoa(size>>20)+"MiB", func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(len(source)))
			for i := 0; i < b.N; i++ {
				if _, err := New().RunString(source); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
