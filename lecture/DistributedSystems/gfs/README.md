# 什么是GFS
GFS是一个可扩展的、高性能的、高可用的分布式文件系统，集成了监控、故障检测、容错以及自动恢复的分布式系统，主要在大文件、顺序IO的场景下使用。

# GFS架构
![](resources/gfs%E6%9E%B6%E6%9E%84.jpg)

GFS将文件拆分成一个或者多个固定大小的`chunk`，每个`chunk`都是`chunkserver`所在机器的本地文件系统下的一个文件。在GFS中，每个`chunk`都由集群全局唯一的`chunkhandle`标识，`master`负责维护文件和`chunkhandle`的映射关系以及`chunkhandle`和其所在`chunkserver`的映射关系。客户端在读文件之前，会首先根据`file name`和`chunk index`请求`master`获取`chunkhandle`和`chunkserver`位置信息，接下来使用`chunkhandle`和读的区域请求对应的`chunkserver`，`chunkserver`通过`chunkhandle`定位文件系统下的某个`chunk`文件，最后会对该`chunk`读取数据。

# GFS主要机制
## operation log
在修改元数据时，需要先在本地和副本写入`operation log`。`operation log`将所有写请求都序列化处理，所有副本都按照`operation log`写入顺序来执行，同时也提供了服务重启后可以通过回放`operation log`来重建状态。

<div style="padding: 10px; background-color: gray; ">
  <p>如果operation log复制到部分副本，论文中说会响应一个错误，但是已经复制到的副本如何处理该operation log论文并没有提及</p>
</div>

## chunk lease
当需要修改`chunk`时，`master`会在该`chunk`的所有副本中选择一个`primary`副本，`master`授予`primary`副本一定时间段的`lease`，在`lease`有效时间段内`primary`副本可以处理客户端的写请求，同时`lease`可以续租。

在`master`触发选择`primary`副本时，`master`会认为接下来对该`chunk`有修改操作，会递增`chunk`的版本号，`chunkserver`持久化该版本号，这样`chunkserver`重启时能够重建状态。版本号的作用区分旧数据，比如`chunkserver`故障导致`chunk`没有被修改，因此其版本号是落后于`master`维护的`chunk`版本号，这样`master`在获取该`chunkserver`的`chunk`时可以标记为垃圾数据。

如果`master`和`primary`副本的`chunkserver`出现网络分区，是否可以在`lease`有效期内重新选择`primary`副本，答案是否定的，因为可能会出现两个`primary`副本做决策，那么会导致写请求出现交叉，导致数据不一致。

## IO
客户端在读取某个文件指定范围内的数据时，首先会根据`chunk`大小计算需要读取的一个或者多个`chunk`（如果有跨`chunk`的读取），接下来根据文件名和`chunk`索引请求`master`节点得到`chunkhandle`列表和`chunkserver`的位置信息，最后按照`chunkhandle`并发向`chunkserver`发起读请求，`chunkserver`根据`chunkhandle`定位本地文件系统下的文件，读取指定范围内的数据响应客户端。

客户端如果需要写某个文件指定范围内的数据，在请求`master`节点获取`chunkhandle`时，`master`会判断是否存在`primary`副本，如果不存在，会触发`primary`副本的选择过程，接下来客户端将数据写入所有副本的`chunkserver`，`chunkserver`缓存该数据，当所有副本都确认收到数据后，客户端向`primary`副本发起写入数据的请求，`primary`副本写入一条`operation log`，并且复制到所有副本。

<div style="padding: 10px; background-color: gray;">
  <p><strong>疑问</strong>：如果客户端在向primary副本发起写入数据时，lease失效如何处理？</p>
  <p><strong>解答</strong>：客户端会重新向master查询chunk的副本信息。</p>

  <p><strong>疑问</strong>：在客户端推送数据到所有副本时，按照什么路径来转发？</p>
  <p><strong>解答</strong>：类似p2p的转发方式，尽量利用好所有chunkserver的出向带宽，客户端首先选择最近的副本推送数据，接下来该副本负责转发数据到其他副本</p> 
</div>

## snapshot
GFS支持对某一个目录或者某个目录树打快照，GFS使用`COW`的方式尽量减少`snapshot`的开销，在执行`snapshot`时，首先需要回收所有`chunk`的`lease`，保证该时间点的写请求都被处理完毕，接下来写入一条`operation log`，`snapshot`首先指向现有的`chunk`，并且增加该`chunk`的引用计数，当`chunk`需要修改时，并且引用计数大于1，那么会触发`chunk`的复制操作，这样后续的写入是在新的`chunk`上执行。

## record append
`record append`是一种追加写，其保证写入的原子性，并且写入的数据能够对所有客户端可见。相比较随机写，`record append`由GFS选择要写入的`offset`。`primary`副本一个`offset`，所有副本在该`offset`写入数据，`offset`是`primary`副本的文件尾。

<div style="padding: 10px; background-color: gray;">
  <p><strong>疑问</strong>：如果文件的最后一个chunk的可用空间不足写入的数据？</p>
  <p><strong>解答</strong>：GFS会该chunk补齐，并且创建额外的chunk，补齐的方式可能类似Linux空洞文件。</p>
</div>

## namespace
当需要修改文件系统元数据时，比如创建、删除文件等。文件系统每个目录或者文件都有一个读写锁，在创建文件时，会获取整个路径上所有目录的读锁，然后获取需要创建的文件的写锁，这样同目录创建可以并发。

## 副本放置、创建、均衡
默认每个`chunk`都由三个副本，GFS每个机架都是一个故障域，三个副本不会分布在同一个故障域。

在创建副本时，为了最大化利用所有`chunkserver`资源，GFS会优先将`chunk`创建在可用容量比较大的`chunkserver`，并且限定一段时间内`chunkserver`创建`chunk`的数量避免大量IO落到同一个`chunkserver`上。

随着时间的推移，`chunkserver`的磁盘空间和吞吐负载都不同，为了让集群是健康的状态，`master`会将磁盘空间比较少或者比较繁忙的`chunkserver`中的`chunk`均衡到其他相对来说资源比较充分的`chunkserver`上，另外，`chunkserver`扩容缩容，磁盘故障或者副本数量出现变化，都会触发副本均衡。

## 垃圾回收
GFS在删除文件时，并不会实际执行删除，而是将文件隐藏，由后台的垃圾回收任务定期扫描文件系统来删除，这么做的好处有：

- 在`chunk`创建时，可能存在部分副本创建成功，这样`master`没有`chunk`信息，可以通过垃圾回收来处理这类孤儿`chunk`
- 垃圾回收可以批量删除多个`chunk`，同时可以对垃圾回收任务限速，避免影响实际的业务请求，使得删除对系统的负载影响最小
- 可以恢复删除的文件

# GFS一致性模型
|名词|含义|
|:-:|:-:|
|consistent|所有客户端读取某个范围都能够读到相同的数据|
|defined|首先是consitent，另外写入的数据能够完整地对所有客户端可见|
|inconsistent|和consitent相反

- 并发的随机写，GFS保证是consistent，但是写入的region可能会导致该region包含并发写入的不同部分。比如两个客户端写入文件的某个范围，这个范围由于垮了chunk的边界，会拆分成两个请求，那么总共有四个请求，记为$C_{0}^{0}$、$C_{0}^{1}$、$C_{1}^{0}$、$C_{1}^{1}$，如果GFS处理的顺序是$C_{0}^{0}$、$C_{1}^{1}$、$C_{1}^{0}$、$C_{0}^{1}$，那么最终包含的结果是$C_{1}^{0}$、$C_{0}^{1}$，这会导致任意一次写入不能完整地被客户端读到。
- record append保证是defined，写入的offset由GFS来选择，因此每次`record append`都会写到同样的chunk的同一个范围内，另外`record append`限定每次最多只能写入chunk大小的1/4，因此不存在跨chunk的写。如下图，应用有三次`record append`，由于在写入B的过程失败后重试，出现了不一致的区域。

    ```
    +---+  +---+  +---+
    | B |  | B |  | B |
    +---+  +---+  +---+
    | C |  | C |  | C |
    +---+  +---+  +---+
    | B |  | B |  |   |
    +---+  +---+  +---+
    | A |  | A |  | A |
    +---+  +---+  +---+
    ```

- 如果写失败，会导致部分副本写入部分副本没有写入，这会导致chunk的副本出现不一致

对于不一致的行为或者重复写入，如上图的第一个副本有一个重复写入的B，应用层可以检查是否是补齐的区域，或者通过写入上加入唯一标识符来识别出是重复写入的数据。而这种重试导致的乱序，应用层需要容忍。
