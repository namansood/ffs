package main

import (
	"encoding/binary"
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
}

type File struct {
	name []byte
	data []byte
	ref  uint64
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

func Metadata(n Node) []byte {
	buf := make([]byte, FILE_METADATA_SIZE)
	idx := 0
	filename := n.Name()
	binary.BigEndian.PutUint16(buf[idx:idx+MAX_NAMESIZE_SIZE], uint16(len(filename)))
	idx += MAX_NAMESIZE_SIZE
	copy(buf[idx:idx+MAX_NAME_SIZE], filename)
	idx += MAX_NAME_SIZE
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

	fileRef := d.getRef() + REF_SIZE + uint64(metadataSize)

	for _, n := range d.nodes {
		n.setRef(fileRef)
		buf = append(buf, Metadata(n)...)
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

func main() {
	root := &Directory{
		parent: nil,
		name:   []byte("root"),
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

	data := root.Data()
	sz := root.Size()

	log.Printf("Data: % x", data)
	log.Printf("Size: %v", sz)
}
