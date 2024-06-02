module kvsrv

go 1.22.3

replace labrpc => ../labrpc

replace porcupine => ../porcupine

replace models => ../models

replace labgob => ../labgob

require (
	labrpc v0.0.0-00010101000000-000000000000
	models v0.0.0-00010101000000-000000000000
	porcupine v0.0.0-00010101000000-000000000000
)

require labgob v0.0.0-00010101000000-000000000000 // indirect
