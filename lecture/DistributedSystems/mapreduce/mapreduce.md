1. 什么是`mapreduce`?

    `mapreduce`是一种编程思想，`map`和`reduce`都是两个用户自定义的函数，函数原型如下：
    ```go
    func Map(key string, val string) []KeyValue
    func Reduce(key string, val []string) string
    ```
    `map`处理一个key和val，并且返回多个中间数据，`reduce`将中间数据排序后，合并同一个key的val列表。

2. `mapreduce`执行流程？



3. 故障出现时的语义（semantics）？
    
    对于确定性的mapreduce任务，那么多次执行其中部分map或者reduce任务结果都会保证是一样的。确定性表示对于同样的输入，多次调用用户定义的`map`和`reduce`函数都会输出同样的结果。`map`的输出文件总会写到临时目录中，当`map`执行完毕，就会将输出的文件列表告知`master`，`master`只有在对应的`map`任务还未执行完才会采纳这些临时文件，否则会拒绝。`reduce`的输出首先写入临时文件，成功执行完毕后会调用一次`rename`，底层文件系统必须保证`rename`的原子性，这样即使多次调用`reduce`，最终只有一份结果。

    对于非确定性的mapreduce任务，mapreduce提供`weaker semantics`，这意味当出现故障时`map`或者`reduce`被执行多次相比较顺序执行（串行执行）`mapreduce`任务，两者的输出结果不一致。`mapreduce`保证多次执行的`map`得到的中间数据总会一致地给所有`reduce`，论文中举了一个反例：当有一个`map`任务和两个`reduce`任务，如果`reduce1`执行完毕后，`map`再次执行，这并不会使得`reduce2`读取的输入变为重新执行`map`后得到的中间数据。
