# lab03

## Speed up system calls

首先需要创建一个物理页，并且将该页映射到进程的地址空间中，并且虚拟地址为`USYSCALL`，这里可以参考[`trapframe`](https://github.com/mit-pdos/xv6-riscv/blob/riscv/kernel/proc.c#L198)的做法。`p->usyscall`是在[`allocproc()`](https://github.com/mit-pdos/xv6-riscv/blob/riscv/kernel/proc.c#L110)函数中调用[`kmalloc()`](https://github.com/mit-pdos/xv6-riscv/blob/riscv/kernel/kalloc.c#L69)分配的物理页。下一步会调用[`proc_pagetable`](https://github.com/mit-pdos/xv6-riscv/blob/riscv/kernel/proc.c#L177)为进程分配页表，并将`p->usyscall`映射到进程地址空间虚拟地址为`USYSCALL`处，后续就可以通过设置`p->usyscall->pid`，这样用户就能够在U-mode下读取到进程的pid了。当需要修改`p->usyscall`的任意成员，都只能通过S-mode，也就是内核态下才能修改。

```c
#ifdef LAB_PGTBL
  if (mappages(pagetable, USYSCALL, PGSIZE,
               (uint64)(p->usyscall), PTE_R | PTE_U) < 0){
    uvmunmap(pagetable, TRAMPOLINE, 1, 0);
    uvmunmap(pagetable, TRAPFRAME,  1, 0);
    uvmfree(pagetable, 0);
    return 0;
  }
#endif
```
> Which other xv6 system call(s) could be made faster using this shared page? Explain how.

如果满足以下特点，那么系统调用就可以通过使用`USYSCALL`映射的页来避免切换到内核态：
- 系统调用的数据只会涉及到`USYSCALL`中的数据，并且只能读取
- 系统调用读取的数据是一旦进程设置后就不会再发生改变或者是系统的全局数据但是不会发生改变

根据以上两个特点，只有`getpid()`满足，其他系统调用都不满足。

## Print a page table
这是一个目标非常明确的任务，打印页表
```c
static char* depth_str[] = {
  "..", ".. ..", ".. .. .."
};

static void 
recursive_vmprint(pagetable_t pagetable, int depth)
{
  for (int i = 0; i < 512; i++){
    pte_t pte = pagetable[i];
    if ((pte & PTE_V) == 0) {
      continue;
    }
    printf("%s%d: pte %p pa %p\n", depth_str[depth], i, pte, PTE2PA(pte));
    if ((pte & (PTE_R|PTE_W|PTE_X)) == 0){
      recursive_vmprint((pagetable_t)(PTE2PA(pte)), depth + 1);
    }
  }
}

void
vmprint(pagetable_t pagetable) {
  printf("page table %p\n", pagetable);
  recursive_vmprint(pagetable, 0);
}
```

> For every leaf page in the vmprint output, explain what it logically contains and what its permission bits are.

```
page table 0x0000000087f6b000
..0: pte 0x0000000021fd9c01 pa 0x0000000087f67000
.. ..0: pte 0x0000000021fd9801 pa 0x0000000087f66000
.. .. ..0: pte 0x0000000021fda01b pa 0x0000000087f68000
.. .. ..1: pte 0x0000000021fd9417 pa 0x0000000087f65000
.. .. ..2: pte 0x0000000021fd9007 pa 0x0000000087f64000
.. .. ..3: pte 0x0000000021fd8c17 pa 0x0000000087f63000
..255: pte 0x0000000021fda801 pa 0x0000000087f6a000
.. ..511: pte 0x0000000021fda401 pa 0x0000000087f69000
.. .. ..509: pte 0x0000000021fdcc13 pa 0x0000000087f73000
.. .. ..510: pte 0x0000000021fdd007 pa 0x0000000087f74000
.. .. ..511: pte 0x0000000020001c0b pa 0x0000000080007000
init: starting sh
```

首先通过一个[python脚本](./parse_vmprint_output.py)解析叶节点的PTE值获取权限位，接下来就可以根据[Fig 3.4](https://pdos.csail.mit.edu/6.828/2023/xv6/book-riscv-rev3.pdf)和权限位判断属于哪个section了。

|va|perm|pa|section|
|:-:|:-:|:-:|:-:|
|0x0|R-XU|0x87f68000|text|
|0x1000|RW-U|0x87f65000|data|
|0x2000|RW--|0x87f64000|guard page|
|0x3000|RW-U|0x87f63000|stack|
|0x3fffffd000|R--U|0x87f73000|usyscall|
|0x3fffffe000|RW--|0x87f74000|trapframe|
|0x3ffffff000|R-X-|0x80007000|trampoline|

## Detect which pages have been accessed

新增一个系统调用可以获取虚拟地址的`A bit`，注意`A bit`需要重置。
```c
int
sys_pgaccess(void)
{
  uint64 va;
  int pages;
  uint64 mask_out;
  unsigned int mask;
  struct proc *p;

  argaddr(0, &va);
  argint(1, &pages);
  argaddr(2, &mask_out);
  p = myproc();

  if (pages > 32) 
    pages = 32;
  for (int i = 0; i < pages; i++){
    pte_t* pte = walk(p->pagetable, va, 0);
    if (*pte & PTE_A) {
      *pte &= ~PTE_A;
      mask |= (1 << i);
    }
    va += PGSIZE;
  }  

  copyout(p->pagetable, mask_out, (char*)&mask, sizeof(mask));

  return 0;
}
```
