# labrpc

labrpc是一个模拟RPC框架的测试框架，支持模拟丢包、高延迟、网络分区。

## Service

Service是一个包含RPC方法的对象。如果一个类的方法满足如下条件，labrpc会将该方法暴露为RPC方法
- 方法名首字母大写，也就是public的方法
- 包含两个参数且第2个参数是指针传递
- 没有返回值

```golang
func (s Xxx) MethodName(args ArgType, reply *ReplyType)
```

labrpc提供函数`MakeService`基于一个对象构造Service对象

```golang
func MakeService(rcvr interface{}) *Service
```

## Server

Server是一个服务进程实体，Server和Service是一对多的关系，也就是一个Server可以包含多个Service对象。ClientEnd在调用RPC方法时，会指定RPC方法名，具体格式为

```
ServiceName.MethodName
```

`ServiceName`是`Service`对象的名字，`MethodName`是方法名，比如调用`Foo::Add`，那么RPC方法名就是`Foo.Add`。

```golang
type Foo struct {}

func (f *Foo) Add(args AddArgs, reply *AddReply) {
    ...
}
```
Server包含一个Service表，通过Service表和ServiceName来路由到具体Service。可以通过`AddService`方法将Service注册到Server中。

```golang
func (rs *Server) AddService(svc *Service)
```

## ClientEnd
ClientEnd表示调用Server的RPC方法，是客户端。ClientEnd包含一个方法，其中第一个参数是RPC方法名，返回值表示本次RPC是否成功。

```golang
func (e *ClientEnd) Call(svcMeth string, args interface{}, reply interface{}) bool
```

## Network

Network是labrpc最复杂的类，Network提供的功能包括：
- 建立ClientEnd和Server的连接
- 将某个Endpoint和当前网络分区
- 模拟网络丢包、重排

下面我们看下Network具体是如何实现上述功能的。

labrpc并没有提供实际的网络库功能，所有包的转发、路由都是由Network模拟的，这意味着Network会维护各个节点之间的连接关系。labrpc使用Server和ClientEnd分别表示服务端和客户端。对于网络或者分布式系统中，一个节点表示的是一个运行实体，如一台物理机、一台虚机或者一个容器。一个节点既可以为另外一个节点提供RPC服务，也可以请求其他节点的RPC方法，在labrpc中前者被抽象为Server，后者使用ClientEnd表示。

- Network是如何模拟网络不稳定，丢包概率增大以及网络发生拥挤的

当设置Network网络不稳定时，Network在处理RPC时增加延时并且会丢弃10%的包

```golang
func (rn *Network) Reliable(yes bool)
```

- Network如何模拟Server重启

如下两个方法，分别是在网络中添加一个Server节点以及移除一个Server节点，当同一个Servername添加两次Server时，那么表示Server发生重启，会将重启之前包都丢弃。
当调用DeleteServer时，那么会将Server节点从网络中移除，那么移除之前正在处理的请求都会丢弃。
```golang
func (rn *Network) AddServer(servername interface{}, rs *Server)
func (rn *Network) DeleteServer(servername interface{})
```

- Network如何模拟请求重排的
如下，设置LongReordering，那么响应会被随机delay一段时间发出去。
```golang
func (rn *Network) LongReordering(yes bool)
```

## Example

```golang
network := MakeNetwork()

// server
xxxService := MakeService(xxx)
yyyService := MakeService(yyy)

server := MakeServer()
server.AddService(xxxService)
server.AddService(yyyService)
network.AddServer("server", server)

// client
client := network.MakeEnd("client")
network.Connect("client", "server")
network.Enable("client", true)
client.Call("xxx.Xxx", args, reply)
```
