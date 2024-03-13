# lab05

## Copy-on-Write Fork for xv6

`fork`会复制父进程的`pagetable`，如果父进程的占用内存很大，复制会耗费大量时间，可以通过copy-on-write来优化，也就是将子进程和父进程共享物理内存，当发生写入时才会实际将写入的物理页拷贝一次。

由于父进程和子进程会引言同一个物理页，因此需要实现引用计数来管理物理页，避免进程退出释放页表时将其他进程需要的物理页给释放。

第一步修改`kalloc.c`，增加引用计数数组
```c
#define REFINDEX(pa) (((pa) - KERNBASE) / PGSIZE)

struct {
  struct spinlock lock;
  struct run *freelist;
  int refcnt[REFINDEX(PHYSTOP)];
} kmem;

void
kref(void* pa)
{
  acquire(&kmem.lock);
  kmem.refcnt[REFINDEX((uint64)pa)]++;
  release(&kmem.lock);
}
```
修改`kalloc`和`kfree`的代码，添加对于引用计数的控制

```c
void
kfree(void* pa)
{
    ...
    ref = kmem.refcnt[REFINDEX((uint64)pa)]--;
    if(ref == 1){
        memset(pa, 1, PGSIZE);
        r->next = kmem.freelist;
        kmem.freelist = r;
    } else if(ref <= 0)
        panic("kfree");
    ...
}

void* 
kalloc(void)
{
    ...
    r = kmem.freelist;
    if(r){
        kmem.freelist = r->next;
        if(kmem.refcnt[REFINDEX((uint64)r)]++ != 0)
        panic("kalloc");
    }
    ...
}
```

接下来就是修改`uvmcopy`，对于具有`PTE_W`并且时用户权限的页，将其写权限清除，并新增`PTE_C(1 << 8)`，这是使用PTE的保留给S-mode使用的位。

```c
int
uvmcopy(pagetable_t old, pagetable_t new, uint64 sz, int cow)
{
    ...
    for(i = 0; i < sz; i += PGSIZE){
        ...
        if(cow){
            if((*pte & PTE_W) && (*pte & PTE_U)){
                *pte &= ~PTE_W;
                *pte |= PTE_C;
            }
            kref((void*)pa);
            mem = (char*)pa;
            flags = PTE_FLAGS(*pte);
        }else{
            if((mem = kalloc()) == 0)
                goto err;
            memmove(mem, (char*)pa, PGSIZE);
        }
        ...
    }
    ...
}
```

修改具有`PTE_C`标志位的页将会触发`storage page fault`，此时`scause`的值为15，首先定义一个新的函数`pagefault`来解决这种异常。`pagefault`函数逻辑很简单，分配新的物理页，并将原来的页内容拷贝，将权限位设值上`PTE_W`，再次回到用户进程就可以正常写入了。

```c
int
pagefault(pagetable_t pagetable, uint64 va)
{
    pte_t *pte;
    void *mem;
    uint64 pa;

    pte = walk(pagetable, va, 0);
    if(pte == 0 || (*pte & PTE_V) == 0 ||
       ((*pte & PTE_U) == 0 && (*pte & PTE_C) == 0)){
        return -1;
    }
    pa = PTE2PA(*pte);
    if((mem = kalloc()) == 0)
        return -1;
    memmove(mem, (void*)pa, PGSIZE);
    *pte = PA2PTE((uint64)mem) | PTE_FLAGS(*pte);
    *pte &= ~PTE_C;
    *pte |= PTE_W;
    kfree((void*)pa);
    return 0;
}
```

最后就是在异常handler`usertrap`中，对于`scause==15`时调用`pagefault`，注意需要判断`stval`的虚拟地址范围，否则`usertests -q`的一些case过不了。

```c
void
usertrap(void)
{
    ...
    if(r_scause() == 8){
        // system call
    }else if(r_scause() == 15){
        if(r_stval() < PGSIZE || r_stval() > p->sz || pagefault(p->pagetable, r_stval()) < 0){
            printf("usertrap(): unexpected scause %p pid=%d\n", r_scause(), p->pid);
            printf("            sepc=%p stval=%p\n", r_sepc(), r_stval());
            setkilled(p);
        }
    }else{
        ...
    }
    ...
}
```
