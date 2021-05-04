# ffs - folksy file system

Filesystem layout:

Directory:

	[
		reference to parent directory (zero for root), [uint64]
		count of number of files, [uint64]
		for each file, file metadata..., [metadata, see format below]
		for each file, file contents..., [binary data]
	]

File metadata:

	[
		size of filename, [uint16]
		bytes of name..., [byte array]
		size of file contents, [uint64]
		ref to file contents, [uint64]
		bool/byte for isDir? [byte]
	]
