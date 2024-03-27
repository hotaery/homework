# locks

## memory allocator
xv6使用一个全局的[`freelist`](https://github.com/mit-pdos/xv6-riscv/blob/riscv/kernel/kalloc.c#L23)来管理物理页的，由于kernel线程会并发读写`freelist`，因此xv6使用[`spinlock`](https://github.com/mit-pdos/xv6-riscv/blob/riscv/kernel/kalloc.c#L22)来保护`freelist`，使用全局的`freelist`会带来较多的锁竞争，导致cpu一直在等`spinlock`而空转。

通过给每一个CPU分配单独的`freelist`可以有效地减少锁竞争，并且在初始化时将物理页平分给每个CPU。新增`cpu_free`函数，`cpu_free`需要传入`cpuid`参数，`cpu_free`根据参数`cpuid`将物理页放到该CPU局部的`freelist`中。

```c
struct {
  struct spinlock lock;
  struct run *freelist;
} kmem[NCPU];

void
kinit()
{
  for (int i = 0; i < NCPU; i++)
    initlock(&kmem[i].lock, "kmem");
  freerange(end, (void*)PHYSTOP);
}

static void
cpu_kfree(void *pa, int id) 
{
  struct run *r;

  if(((uint64)pa % PGSIZE) != 0 || (char*)pa < end || (uint64)pa >= PHYSTOP)
    panic("kfree");

  // Fill with junk to catch dangling refs.
  memset(pa, 1, PGSIZE);

  r = (struct run*)pa;

  acquire(&kmem[id].lock);
  r->next = kmem[id].freelist;
  kmem[id].freelist = r;
  release(&kmem[id].lock);
}

void
freerange(void *pa_start, void *pa_end)
{
  char *p;
  int id;
  p = (char*)PGROUNDUP((uint64)pa_start);
  id = 0;
  for(; p + PGSIZE <= (char*)pa_end; p += PGSIZE, id = (id + 1) % NCPU)
    cpu_kfree(p, id); 
}
```

在分配内存时，优先使用CPU本身的`freelist`，如果该CPU的`freelist`没有可用的物理页了，就采用窃取其他CPU的`freelist`来获取可用的物理页。首先新增`cpu_alloc`函数，`cpu_alloc`根据参数`cpuid`从该CPU局部的`freelist`分配可用的物理页。

```c
static void *
cpu_kalloc(int id)
{
  struct run *r;

  acquire(&kmem[id].lock);
  r = kmem[id].freelist;
  if(r)
    kmem[id].freelist = r->next;
  release(&kmem[id].lock);

  if(r)
    memset((char*)r, 5, PGSIZE); // fill with junk
  return (void*)r;
}
```

接下来就是改写`kalloc`函数，需要注意的是，由于中断发生时，会将CPU执行的进程切出去，导致进程重新执行时`cpuid()`发生变化，可能错过某些`freelist`导致分配不到物理页。比如，进程刚开始执行在0号CPU上，并且0号CPU的`freelist`没有可用的物理页了，由于timer interrupt，进程切到3号CPU了，在分配内存时跳过3号CPU的`freelist`，`kalloc`有可能返回`NULL`，因此需要使用`push_off`和`pop_off`来关闭中断和打开中断。
```c
void *
kalloc(void)
{
  void *pa;
  int i;

  push_off();
  i = cpuid();
  pa = cpu_kalloc(i);
  for(i = 0; i < NCPU && !pa; i++)
  {
    if(i == cpuid())
      continue;
    pa = cpu_kalloc(i);
  } 
  pop_off();
  return pa;
}
```

和`kalloc`类似，`kfree`也需要改写
```c
void
kfree(void *pa)
{
  push_off();
  cpu_kfree(pa, cpuid());
  pop_off();
}
```

## buffer cache
`buffer cache`作为文件系统的`cache layer`，其提供两点功能：
    
- 缓存经常访问的block，避免重新从较慢的磁盘读取
- 保证磁盘的每个block只会有一份缓存

和`memory allocator`相同的点是：固定大小的资源池需要同步多个进程同时访问，不同的点是，`buffer cache`需要提供查找的功能，因此不能像`memory allocator`每个CPU都有自己局部的`freelist`来避免锁冲突。

可以减少临界区的大小以及锁的粒度尽可能地降低锁冲突。通过使用固定bucket数的哈希表，只要bucket没有被同时访问，就可以避免锁冲突，这是通过减小锁粒度来降低锁冲突发生的可能性。如下代码，将[`struct buf`](https://github.com/mit-pdos/xv6-riscv/blob/riscv/kernel/buf.h#L1)的`prev`和`next`移除，使用`struct node`代替，这是因为每个bucket都需要有额外的头节点，而`struct buf`携带数据，会占用`BSIZE`的空间，并且头节点是不会被使用的，因此会浪费宝贵的内存。每个bucket都包含一个双链表，使用LRU作为淘汰策略。

```c
#define NBUCKET 13
#define HASH(dev, blockno) ((((dev)<<27)|(blockno))%NBUCKET)

struct node {
  struct node *prev;
  struct node *next;
  void *data;
};

struct bucket {
  struct spinlock lock;
  struct node head;  // LRU
};

struct {
  struct spinlock lock;
  struct buf buf[NBUF];
  struct node bufnode[NBUF];
  struct bucket bucket[NBUCKET];
  uint   lastbkt;
} bcache;
```

接下来就是初始化`bcache`，除了初始化锁之外，需要将`bcache.bufnode`初始化为循环双链表，并且放到第0个bucket中，因为初始化时`dev`和`blockno`全为0。
```c
void
binit(void)
{
  int i;
  struct node *node;
  struct buf *b;

  initlock(&bcache.lock, "bcache");
  for(i = 0; i < NBUCKET; i++){
    initlock(&bcache.bucket[i].lock, "bcache:bucket");
    bcache.bucket[i].head.prev = &bcache.bucket[i].head;
    bcache.bucket[i].head.next = &bcache.bucket[i].head;
  }

  // construct bcache.buf to linked-list and move
  // to first bucket
  for(node = &bcache.bufnode[0], b = &bcache.buf[0]; node < &bcache.bufnode[NBUF]; node++, b++){
    node->data = b;
    node->prev = &bcache.bucket[0].head;
    node->next = bcache.bucket[0].head.next;
    bcache.bucket[0].head.next->prev = node;
    bcache.bucket[0].head.next = node;
    initsleeplock(&b->lock, "buffer");
  }
}
```

`bget`会检查block是否已经被缓存，如果没有被缓存，就需要先找到一个空闲的缓存，这里需要注意避免死锁，我这里使用double check来避免死锁以及重复插入。提供两个辅助函数`getfreenode`和`freenode`，前者从所有的bucket中寻找第一个空闲的`struct node`，后者是释放一个`struct node`，调用

```c
static struct node *
getfreenode()
{
  struct node *node;
  struct buf *b;
  uint i = 0;
  uint last = __sync_fetch_and_add(&(bcache.lastbkt), 1);

  for(i = 0; i < NBUCKET; i++)
  {
    acquire(&bcache.bucket[(last+i)%NBUCKET].lock);
    node = bcache.bucket[(last+i)%NBUCKET].head.prev;
    while(node != &bcache.bucket[(last+i)%NBUCKET].head){
      b = (struct buf*)node->data;
      if(b->refcnt == 0){
        node->prev->next = node->next;
        node->next->prev = node->prev;
        release(&bcache.bucket[(last+i)%NBUCKET].lock);
        return node;
      }
      node = node->prev;
    }
    release(&bcache.bucket[(last+i)%NBUCKET].lock);
  }
  return 0;
}

static void
freenode(struct node *node)
{
  struct buf *b;
  uint bid;

  if(!node)
    return;
  b = (struct buf*)node->data;
  bid = HASH(b->dev, b->blockno);
  acquire(&bcache.bucket[bid].lock);
  if(b->refcnt != 0)
    panic("freenode: refcnt not equal to 0");
  node->prev = bcache.bucket[bid].head.prev;
  node->next = &bcache.bucket[bid].head;
  bcache.bucket[bid].head.prev->next = node;
  bcache.bucket[bid].head.prev = node;
  release(&bcache.bucket[bid].lock);
}

// Look through buffer cache for block on device dev.
// If not found, allocate a buffer.
// In either case, return locked buffer.
static struct buf*
bget(uint dev, uint blockno)
{
  struct node *node, *node2;
  int bid;
  struct buf *b;

  bid = HASH(dev, blockno);
  b = 0;
  acquire(&bcache.bucket[bid].lock);

  // Is the block already cached?
  for(node = bcache.bucket[bid].head.next; node != &bcache.bucket[bid].head; node = node->next){
    b = (struct buf*)node->data;
    if(b->dev == dev && b->blockno == blockno){
      b->refcnt++;
      release(&bcache.bucket[bid].lock);
      acquiresleep(&b->lock);
      return b;
    }
  }
  release(&bcache.bucket[bid].lock);
  node2 = getfreenode();
  acquire(&bcache.bucket[bid].lock);
  
  // double check
  for(node = bcache.bucket[bid].head.next; node != &bcache.bucket[bid].head; node = node->next){
    b = (struct buf*)node->data;
    if(b->dev == dev && b->blockno == blockno){
      b->refcnt++;
      release(&bcache.bucket[bid].lock);
      freenode(node2);
      acquiresleep(&b->lock);
      return b;
    }
  }

  if(node2){
    b = (struct buf*)node2->data;
    if(b->refcnt != 0)
      panic("bget: refcnt not equal to 0");
    b->dev = dev;
    b->blockno = blockno;
    b->valid = 0;
    b->refcnt = 1;
    node2->prev = &bcache.bucket[bid].head;
    node2->next = bcache.bucket[bid].head.next;
    bcache.bucket[bid].head.next->prev = node2;
    bcache.bucket[bid].head.next = node2;
    release(&bcache.bucket[bid].lock);
    acquiresleep(&b->lock);
    return b;
  }
  
  panic("bget: no buffers");
}
```

最后就是`brelse`函数了，和原来的版本基本一致。
```c
void
brelse(struct buf *b)
{
  struct node *node;
  uint bid;
  if(!holdingsleep(&b->lock))
    panic("brelse");

  bid = HASH(b->dev, b->blockno);
  node = bcache.bufnode + (b - bcache.buf);
  if(node->data != b)
    panic("brelse: internal error");
  releasesleep(&b->lock);

  acquire(&bcache.bucket[bid].lock);
  b->refcnt--;
  if (b->refcnt == 0) {
    // no one is waiting for it.
    node->next->prev = node->prev;
    node->prev->next = node->next;
    node->next = bcache.bucket[bid].head.next;
    node->prev = &bcache.bucket[bid].head;
    bcache.bucket[bid].head.next->prev = node;
    bcache.bucket[bid].head.next = node;
  }
  
  release(&bcache.bucket[bid].lock);
}
```

另外，也需要修改`bpin`和`bunpin`，否则`bcachetest`可能过不了
```c
void
bpin(struct buf *b) {
  acquire(&bcache.lock);
  b->refcnt++;
  release(&bcache.lock);
}

void
bunpin(struct buf *b) {
  acquire(&bcache.lock);
  b->refcnt--;
  release(&bcache.lock);
}
```