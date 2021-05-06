package main

import (
	"encoding/binary"
	"fmt"
	"log"
)

const (
	MAX_NAMESIZE_SIZE  = 2 // size of uint16
	MAX_NAME_SIZE      = 32
	MAX_FILE_SIZE      = 8 // size of uint64
	MAX_ISDIR_SIZE     = 1
	REF_SIZE           = 8 // size of uint64
	FILE_METADATA_SIZE = MAX_NAMESIZE_SIZE + MAX_NAME_SIZE + MAX_FILE_SIZE + REF_SIZE + MAX_ISDIR_SIZE

	MAX_FILECOUNT_SIZE = 8 // size of uint64
)

type Node interface {
	Name() []byte
	Size() uint64
	Data() []byte
	IsDir() bool
	getRef() uint64
	setRef(uint64)
	fmt.Stringer
}

type File struct {
	name []byte
	data []byte
	ref  uint64
}

func (f *File) String() string {
	return fmt.Sprintf("%s: % x", string(f.name), f.data)
}

func (f *File) Name() []byte {
	return f.name
}

func (f *File) Size() uint64 {
	return uint64(len(f.data))
}

func (f *File) Data() []byte {
	return f.data
}

func (f *File) IsDir() bool {
	return false
}

func (f *File) getRef() uint64 {
	return f.ref
}

func (f *File) setRef(v uint64) {
	f.ref = v
}

func MetadataBlobFromNode(n Node) []byte {
	buf := make([]byte, FILE_METADATA_SIZE)
	idx := 0
	filename := n.Name()
	filenameLen := uint16(len(filename))
	binary.BigEndian.PutUint16(buf[idx:idx+MAX_NAMESIZE_SIZE], filenameLen)
	idx += MAX_NAMESIZE_SIZE
	copy(buf[idx:idx+int(filenameLen)], filename)
	idx += int(filenameLen)
	binary.BigEndian.PutUint64(buf[idx:idx+MAX_FILE_SIZE], n.Size())
	idx += MAX_FILE_SIZE
	binary.BigEndian.PutUint64(buf[idx:idx+REF_SIZE], n.getRef())
	idx += REF_SIZE
	if n.IsDir() {
		buf[idx] = 1
	} else {
		buf[idx] = 0
	}

	return buf
}

type Directory struct {
	parent *Directory
	name   []byte
	nodes  []Node
	ref    uint64

	precomputedData []byte
}

func (d *Directory) String() string {
	out := string(d.name) + ": { "
	for _, n := range d.nodes {
		out += n.String() + ", "
	}
	out += "}"
	return out
}

func (d *Directory) Name() []byte {
	return d.name
}

func (d *Directory) Size() uint64 {
	if d.precomputedData == nil {
		d.Data()
	}
	return uint64(len(d.precomputedData))
}

func (d *Directory) Data() []byte {
	if d.precomputedData != nil {
		return d.precomputedData
	}

	buf := make([]byte, REF_SIZE+MAX_FILECOUNT_SIZE)
	if d.parent != nil {
		binary.BigEndian.PutUint64(buf, d.parent.getRef())
	}
	binary.BigEndian.PutUint64(buf[REF_SIZE:], uint64(len(d.nodes)))

	metadataSize := len(d.nodes) * FILE_METADATA_SIZE

	fileRef := d.getRef() + REF_SIZE + MAX_FILECOUNT_SIZE + uint64(metadataSize)

	for _, n := range d.nodes {
		n.setRef(fileRef)
		buf = append(buf, MetadataBlobFromNode(n)...)
		fileRef += n.Size()
	}

	for _, n := range d.nodes {
		buf = append(buf, n.Data()...)
	}

	d.precomputedData = buf
	return buf
}

func (d *Directory) IsDir() bool {
	return true
}

func (d *Directory) getRef() uint64 {
	return d.ref
}

func (d *Directory) setRef(v uint64) {
	d.ref = v
}

func ParseNodeFromBlob(metadata []byte, rootBuf []byte, parent *Directory) (Node, error) {
	idx := 0
	filenameLen := int(binary.BigEndian.Uint16(metadata[idx : idx+MAX_NAMESIZE_SIZE]))
	idx += MAX_NAMESIZE_SIZE
	filename := metadata[idx : idx+filenameLen]
	idx += filenameLen
	filesize := binary.BigEndian.Uint64(metadata[idx : idx+MAX_FILE_SIZE])
	idx += MAX_FILE_SIZE
	ref := binary.BigEndian.Uint64(metadata[idx : idx+REF_SIZE])
	idx += REF_SIZE
	// isDir == false
	if metadata[idx] == 0 {
		return &File{
			name: filename,
			data: rootBuf[ref : ref+filesize],
			ref:  ref,
		}, nil
	}

	return ParseDirectory(rootBuf[ref:ref+filesize], rootBuf, parent, filename, ref)
}

func ParseRoot(data []byte) (*Directory, error) {
	return ParseDirectory(data, data, nil, []byte("ffsroot"), 0)
}

func ParseDirectory(data []byte, rootBuf []byte, parent *Directory, name []byte, ref uint64) (*Directory, error) {
	dir := &Directory{
		parent: parent,
		name:   name,
		ref:    ref,
	}

	idx := 0
	parentRef := binary.BigEndian.Uint64(data[idx : idx+REF_SIZE])
	idx += REF_SIZE

	if parent != nil && parent.getRef() != parentRef {
		return nil, fmt.Errorf("parent ref %v does not match parent %s", parentRef, parent.name)
	}

	fileCount := binary.BigEndian.Uint64(data[idx : idx+MAX_FILECOUNT_SIZE])
	idx += MAX_FILECOUNT_SIZE

	nodes := make([]Node, fileCount)

	var i uint64
	for i = 0; i < fileCount; i++ {
		mdBytes := data[idx : idx+FILE_METADATA_SIZE]
		idx += FILE_METADATA_SIZE
		node, err := ParseNodeFromBlob(mdBytes, rootBuf, dir)
		if err != nil {
			return nil, fmt.Errorf("could not read file from directory %s, err %v", string(name), err)
		}
		nodes[i] = node
	}

	dir.nodes = nodes

	return dir, nil
}

func main() {
	root := &Directory{
		parent: nil,
		name:   []byte("ffsroot"),
		nodes:  nil,
		ref:    0,
	}

	foo := &Directory{
		parent: root,
		name:   []byte("foo"),
		nodes: []Node{
			&File{
				name: []byte("hello.txt"),
				data: []byte("Hello, world!\n"),
			},
		},
	}

	bar := &Directory{
		parent: root,
		name:   []byte("bar"),
		nodes:  nil,
	}

	root.nodes = []Node{foo, bar}

	log.Printf("oldRoot: %s", root)

	data := root.Data()
	sz := root.Size()

	log.Printf("Data: % x", data)
	log.Printf("Size: %v", sz)

	newRoot, err := ParseRoot(data)
	if err != nil {
		panic(err)
	}

	log.Printf("newRoot: %s", newRoot)
}
