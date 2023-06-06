# MapReduce

## 什么是`MapReduce`
为了更好地理解`MapReduce`，我们从现代高级编程语言入手。在现代编程语言中都会包含两个函数：`Map`和`Reduce`。在`python`中这两个函数原型如下：
```
map(function, iterable)
reduce(function, iterable[, initializer])
```
`map`接受两个参数，分别是一个函数对象和一个迭代器对象，`map`将函数作用于迭代器的每个元素后返回一个迭代器对象。`reduce`同样接受两个参数，`reduce`函数将函数对象递归的作用于迭代器的每个元素，最终返回一个迭代器元素类型的值。下面是一个简答的`map`和`reduce`使用例子，使用`map`函数将数组的每个元素都乘以2，使用`reduce`函数计算数组每个元素的累加和。
```python
>>> from functools import reduce
>>> L=[1,2,3]
>>> print(list(map(lambda x: x*2, L)))
[2, 4, 6]
>>> print(reduce(lambda x,y: x+y, L))
6
```
从更抽象的角度来看，`map`函数处理列表的每个元素，`reduce`合并列表元素。在`MapReduce`中同样包含两个函数，原型如下：
```golang
func Map(key string, val string) []KeyValue
func Reduce(key string, val []string) string
```
`Map`处理一对kv并且返回中间数据的kv列表，`Reduce`合并同一个key的中间数据。比如在推荐场景中使用的倒排索引，其索引的key是某些关键字，而value是推荐物料的列表。`Map`的key是推荐物料，`val`是可以关键字，那么`Map`就会生成`<target, source>`列表，其中`target`是关键字，`source`是物料。`Reduce`处理同一个关键字的物料列表，返回最终的倒排索引`<target, list<source>>`。

`MapReduce`是在大数据的场景，任务逻辑比较简单，比如倒排索引、单词计数等。由于数据规模比较大，需要将任务拆分成多个独立的子任务，并且将这些子任务在一个集群下执行，尽量将任务运行的总时间压缩在一个合理的范围内。因此是一个实现完整的`MapReduce`需要包括数据切分、任务调度、失败重试等。

## `MapReduce`实现
`MapReduce`将输入数据拆分`M`个分片。接下来在集群中启动多个进程，其中一个`master`进程管理任务运行时的资源调度，失败处理等，其他进程是`worker`，每个`worker`是被动的，执行`master`分配的任务。总共需要执行`M`个`Map`任务和`R`个`Reduce`任务。当执行`Map`任务时，`worker`读取分片的数据，解析数据得到一个或者多个kv，将用户自定义的`Map`作用于每个kv，生成中间数据。中间数据会定期写到`R`个文件，根据中间数据的key哈希到`R`的文件中的某一个并且写入。当执行`Reduce`任务时，`Reduce`任务读取所有`Map`任务生成的中间数据其中一份$R_{i}$，在读取`M`份数据后，将所有中间数据按照`key`排序，`Reduce`函数作用于同一个key的value列表，并且将输出写到最终输出文件中。

在整个`MapReduce`执行完毕后，会有`R`个输出文件。

## 失败处理
由于集群规模比较大，如果出现`master`或者`worker`失败，`MapReduce`需要正确处理这种失败。
`master`会周期性地发心跳到所有`worker`，如果有`worker`失败，`master`将该`worker`是执行过的`Map`任务标记为`IDLE`等待重新调度，注意这里即使是已经执行完`Map`任务也会重新调度，这是因为中间数据是在`worker`机器上，如果`worker`挂掉，中间数据也会变得不可读取，因此需要重新执行`Map`任务，而对于`Reduce`任务，除了在这台`worker`执行的`Reduce`任务需要重新运行，已经执行过的`Reduce`任务不需要重新运行，这是因为`Reduce`输出是写在一个分布式文件系统中。

如果`master`失败，理论上可以通过将`master`的执行状态写到快照中，这样`master`失败可以在这次快照重新，继续执行`MapReduce`，但是可以通过更加简单暴力的方式来处理，直接通知用户`MapReduce`失败，用户重试即可。

失败会导致任务执行多次，如何保证`MapReduce`和单线程顺序执行`MapReduce`的一致性一样？

对于确定性的`MapReduce`任务，那么多次执行并不会造成一致性问题，这是因为`Map`和`Reduce`的输入是相同的，，那么输出也是一样的。`Map`的执行结果是`Reduce`的输入，因此`Reduce`任务的输入也是一致的，如果一个`Reduce`执行多次，`Reduce`首先写入的是临时文件，只有执行结束后才会执行一次`rename`，底层文件系统需要保证`rename`的原子性。

对于不确定性的`MapReduce`任务，`MapReduce`提供一种弱的一致性语义，比如有一个`Reduce`执行完毕，生成$R_{1}$的输出，由于`worker`出多导致`Map`再次执行，此时未执行完毕的`Reduce`就会读取新的`Map`的输出，这导致$R_{1}$和$R_{i}$的输入可能不是同一个`Map`的输出。

