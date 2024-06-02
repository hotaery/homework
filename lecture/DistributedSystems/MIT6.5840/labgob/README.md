# labgob 

labgod是[6.5840](https://pdos.csail.mit.edu/6.824/schedule.html)的公共模块，提供了内存对象的序列化和反序列化能力。

## LabEncoder

`LabEncoder`提供序列化功能的对象，其提供两个方法

- `func (enc *LabEncoder) Encode(e interface{}) error`
- `func (enc *LabEncoder) EncodeValue(value reflect.Value) error`

调用上面两个方法需要满足序列化的对象的每个字段都必须是public的，在golang中，也就是字段名首字母必须大写。另外，容器的元素类型也必须满足上述要求，这是为了避免反序列化的对象不一致。

`LabEncoder`使用下面函数来构造

```golang
func NewEncoder(w io.Writer) *LabEncoder
```

## LabDecoder

`LabDecoder`提供反序列化功能的对象，其提供的方法

```golang
func (dec *LabDecoder) Decode(e interface{}) error
```

调用`Decode`要求对象类型满足所有字段都是public的，传入的参数`e`必须是重新生成的，也就是每次调用`Decode`都需要生成一个所有字段都是默认值的对象，避免部分字段不属于反序列化赋值的。

## Register
`labgob`基于golang标准库的[`encoding/gob`](https://pkg.go.dev/encoding/gob)，`encoding/gob`对于接口类型在执行Encode\Decode之前必须先将类型信息注册到`encoding/gob`中，因此labgob提供两个注册相关的函数

```golang
func Register(value interface{})
func RegisterName(name string, value interface{})
```