package ssz

type Hash []byte

type Metadata struct {
	Version    uint8
	CodeHash   Hash `ssz-size:"32"`
	CodeLength uint16
}

type Chunk32 struct {
	FIO  uint8
	Code []byte `ssz-size:"32"` // Last chunk is right-padded with zeros
}

type Chunk24 struct {
	FIO  uint8
	Code []byte `ssz-size:"24"` // Last chunk is right-padded with zeros
}

type Chunk40 struct {
	FIO  uint8
	Code []byte `ssz-size:"40"` // Last chunk is right-padded with zeros
}

type CodeTrie32 struct {
	Metadata *Metadata
	Chunks   []*Chunk32 `ssz-max:"1024"`
}

type CodeTrie24 struct {
	Metadata *Metadata
	Chunks   []*Chunk24 `ssz-max:"1024"`
}

type CodeTrie40 struct {
	Metadata *Metadata
	Chunks   []*Chunk40 `ssz-max:"1024"`
}
