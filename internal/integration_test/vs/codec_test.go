//go:build amd64 && cgo && !windows

// Wasmtime can only be used in amd64 with CGO
// Wasmer doesn't link on Windows
package vs

import (
	_ "embed"
	"testing"

	"github.com/bytecodealliance/wasmtime-go"
	"github.com/wasmerio/wasmer-go/wasmer"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/internal/testing/require"
	"github.com/tetratelabs/wazero/internal/wasm"
	"github.com/tetratelabs/wazero/internal/wasm/binary"
	"github.com/tetratelabs/wazero/internal/wasm/text"
	"github.com/tetratelabs/wazero/wasi"
)

// example holds the latest supported features as described in the comments of exampleText
var example = newExample()

// exampleText is different from exampleWat because the parser doesn't yet support all features.
//go:embed testdata/example.wat
var exampleText []byte

// exampleBinary is the exampleText encoded in the WebAssembly 1.0 binary format.
var exampleBinary = binary.EncodeModule(example)

func newExample() *wasm.Module {
	three := wasm.Index(3)
	i32, i64 := wasm.ValueTypeI32, wasm.ValueTypeI64
	return &wasm.Module{
		TypeSection: []*wasm.FunctionType{
			{Params: []wasm.ValueType{i32, i32}, Results: []wasm.ValueType{i32}},
			{},
			{Params: []wasm.ValueType{i32, i32, i32, i32}, Results: []wasm.ValueType{i32}},
			{Params: []wasm.ValueType{i64}, Results: []wasm.ValueType{i64}},
			{Params: []wasm.ValueType{i32, i32}, Results: []wasm.ValueType{i32, i32}},
		},
		ImportSection: []*wasm.Import{
			{
				Module: "wasi_snapshot_preview1", Name: "args_sizes_get",
				Type:     wasm.ExternTypeFunc,
				DescFunc: 0,
			}, {
				Module: "wasi_snapshot_preview1", Name: "fd_write",
				Type:     wasm.ExternTypeFunc,
				DescFunc: 2,
			},
		},
		FunctionSection: []wasm.Index{wasm.Index(1), wasm.Index(1), wasm.Index(0), wasm.Index(3), wasm.Index(4)},
		CodeSection: []*wasm.Code{
			{Body: []byte{wasm.OpcodeCall, 3, wasm.OpcodeEnd}},
			{Body: []byte{wasm.OpcodeEnd}},
			{Body: []byte{wasm.OpcodeLocalGet, 0, wasm.OpcodeLocalGet, 1, wasm.OpcodeI32Add, wasm.OpcodeEnd}},
			{Body: []byte{wasm.OpcodeLocalGet, 0, wasm.OpcodeI64Extend16S, wasm.OpcodeEnd}},
			{Body: []byte{wasm.OpcodeLocalGet, 1, wasm.OpcodeLocalGet, 0, wasm.OpcodeEnd}},
		},
		MemorySection: &wasm.Memory{Min: 1, Max: three, IsMaxEncoded: true},
		ExportSection: []*wasm.Export{
			{Name: "AddInt", Type: wasm.ExternTypeFunc, Index: wasm.Index(4)},
			{Name: "", Type: wasm.ExternTypeFunc, Index: wasm.Index(3)},
			{Name: "mem", Type: wasm.ExternTypeMemory, Index: wasm.Index(0)},
			{Name: "swap", Type: wasm.ExternTypeFunc, Index: wasm.Index(6)},
		},
		StartSection: &three,
		NameSection: &wasm.NameSection{
			ModuleName: "example",
			FunctionNames: wasm.NameMap{
				{Index: wasm.Index(0), Name: "wasi.args_sizes_get"},
				{Index: wasm.Index(1), Name: "wasi.fd_write"},
				{Index: wasm.Index(2), Name: "call_hello"},
				{Index: wasm.Index(3), Name: "hello"},
				{Index: wasm.Index(4), Name: "addInt"},
				{Index: wasm.Index(6), Name: "swap"},
			},
			LocalNames: wasm.IndirectNameMap{
				{Index: wasm.Index(1), NameMap: wasm.NameMap{
					{Index: wasm.Index(0), Name: "fd"},
					{Index: wasm.Index(1), Name: "iovs_ptr"},
					{Index: wasm.Index(2), Name: "iovs_len"},
					{Index: wasm.Index(3), Name: "nwritten_ptr"},
				}},
				{Index: wasm.Index(4), NameMap: wasm.NameMap{
					{Index: wasm.Index(0), Name: "value_1"},
					{Index: wasm.Index(1), Name: "value_2"},
				}},
			},
		},
	}
}

func TestExampleUpToDate(t *testing.T) {
	t.Run("binary.DecodeModule", func(t *testing.T) {
		m, err := binary.DecodeModule(exampleBinary, wasm.FeaturesFinished, wasm.MemoryMaxPages)
		require.NoError(t, err)
		require.Equal(t, example, m)
	})

	t.Run("text.DecodeModule", func(t *testing.T) {
		m, err := text.DecodeModule(exampleText, wasm.FeaturesFinished, wasm.MemoryMaxPages)
		require.NoError(t, err)
		require.Equal(t, example, m)
	})

	t.Run("Executable", func(t *testing.T) {
		r := wazero.NewRuntimeWithConfig(wazero.NewRuntimeConfig().WithFinishedFeatures())

		// Add WASI to satisfy import tests
		wm, err := wasi.InstantiateSnapshotPreview1(testCtx, r)
		require.NoError(t, err)
		defer wm.Close()

		// Decode and instantiate the module
		module, err := r.InstantiateModuleFromCode(testCtx, exampleBinary)
		require.NoError(t, err)
		defer module.Close()

		// Call the swap function as a smoke test
		results, err := module.ExportedFunction("swap").Call(testCtx, 1, 2)
		require.NoError(t, err)
		require.Equal(t, []uint64{2, 1}, results)
	})
}

func BenchmarkCodecExample(b *testing.B) {
	b.Run("binary.DecodeModule", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			if _, err := binary.DecodeModule(exampleBinary, wasm.FeaturesFinished, wasm.MemoryMaxPages); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("binary.EncodeModule", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_ = binary.EncodeModule(example)
		}
	})
	b.Run("text.DecodeModule", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			if _, err := text.DecodeModule(exampleText, wasm.FeaturesFinished, wasm.MemoryMaxPages); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("wat2wasm via text.DecodeModule->binary.EncodeModule", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			if m, err := text.DecodeModule(exampleText, wasm.FeaturesFinished, wasm.MemoryMaxPages); err != nil {
				b.Fatal(err)
			} else {
				_ = binary.EncodeModule(m)
			}
		}
	})
	// Note: We don't know if wasmer.Wat2Wasm encodes the custom name section or not.
	// Note: wasmer.Wat2Wasm calls wasmer via CGO which is eventually implemented by wasm-tools
	b.Run("wat2wasm vs wasmer.Wat2Wasm", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_, err := wasmer.Wat2Wasm(string(exampleText))
			if err != nil {
				panic(err)
			}
		}
	})
	// Note: We don't know if wasmtime.Wat2Wasm encodes the custom name section or not.
	// Note: wasmtime.Wat2Wasm calls wasmtime via CGO which is eventually implemented by wasm-tools
	b.Run("wat2wasm vs wasmtime.Wat2Wasm", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_, err := wasmtime.Wat2Wasm(string(exampleText))
			if err != nil {
				panic(err)
			}
		}
	})
}
