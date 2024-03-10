# Q&A

## Which registers contain arguments to functions? For example, which register holds 13 in main's call to printf?

调用`printf`的C代码

```c
printf("%d %d\n", f(8)+1, 13);
```

得到的汇编代码为，其中

- `li a2,13`：表示将13保存到寄存器`a2`中
- `li a1,12`：表示将12保存到寄存器`a1`中
- `auipc a0,0x0`：表示将`pc`保存到寄存器`a0`，更通用的是，将`pc+0x0`保存到寄存器`a0`中
- `addi a0,a0,1992`：表示将0x7f0的数据保存到寄存器`a0`中，0x7f0为程序的只读数据段，因此这里保存的是字符串`%d %d\n'
- `auipc ra,0x0`：表示将`pc`保存到返回地址的寄存器`ra`中
- `jalr 1562(ra)`：跳转到ra+1562处，也就是调用printf

```asm

  24:	4635                li	a2,13
  26:	45b1                li	a1,12
  28:	00000517          	auipc	a0,0x0
  2c:	7c850513          	addi	a0,a0,1992 # 7f0 <malloc+0xe8>
  30:	00000097          	auipc	ra,0x0
  34:	61a080e7          	jalr	1562(ra) # 64a <printf>
  ...

  000000000000064a <printf>:

  00000000000007f0 .rodata
```

因此调用`printf`总共有三个参数，放在`a0 a1 a2`三个寄存器中。

## Where is the call to function f in the assembly code for main? Where is the call to g? 

根据第一问的反汇编代码可以看到有一条代码为，相当于将`f()`内联了

```
  26:	45b1                li	a1,12
```
可以通过强制编译器不要内联

```c
__attribute__((noinline)) 
int g(int x) {
  return x+3;
}

__attribute__((noinline)) 
int f(int x) {
  return g(x);
}
```

得到的反汇编代码为，`jalr`即为跳转到函数`f`或者`g`处。

```
  16:	00000097          	auipc	ra,0x0
  1a:	fea080e7          	jalr	-22(ra) # 0 <g>

  2e:	4521                li	a0,8
  30:	00000097          	auipc	ra,0x0
  34:	fde080e7          	jalr	-34(ra) # e <f>
```

## At what address is the function printf located?


```
  30:	00000097          	auipc	ra,0x0
  34:	61a080e7          	jalr	1562(ra) # 64a <printf>
  ...

    000000000000064a <printf>:
```

## What value is in the register ra just after the jalr to printf in main?

在跳转到`printf`之前，设置`ra=pc`，也就是`0x30`，当调用`jalr`后会带来副作用将`ra += 8`，因此`ra`是`jalr`下一条指令。

## Run the following code.

```c
	unsigned int i = 0x00646c72;
	printf("H%x Wo%s", 57616, &i);
```
- What is the output? 

    ```
    HE110 World
    ```

    `%x`表示使用十六进制打印，因此输出为`0xE110`

    `%s`表示打印字符串，由于`unsigned int`占四个字节，并且采用小端表示，那么从低地址到高地址`0x00646c72`为

    ```
    72  6c  64  00
    ```

    根据[ASCII表](https://www.asciitable.com/)

    |hex|char|
    |:-:|:-:|
    |72|r|
    |6c|l|
    |64|d|

- The output depends on that fact that the RISC-V is little-endian. If the RISC-V were instead big-endian what would you set i to in order to yield the same output? Would you need to change 57616 to a different value?

    如果采用大端表示，那么从低地址到高地址为，`0x00646c72`的表示为

    ```
    00 64 6c 72
    ```

    这会被解析为空字符串，第一个字节为`\0`，因此如果需要输出一样，那么为
    ```
    0x726c6400

    72 6c 64 00
    ```

## In the following code, what is going to be printed after 'y='? (note: the answer is not a specific value.) Why does this happen?

```c
    printf("x=%d y=%d", 3);
```

编译器会将`a2`寄存器的值作为第二%d的参数打印。

## backtrace

函数调用之间栈帧为

```
Stack
                   .
                   .
      +->          .
      |   +-----------------+   |
      |   | return address  |   |
      |   |   previous fp ------+
      |   | saved registers |
      |   | local variables |
      |   |       ...       | <-+
      |   +-----------------+   |
      |   | return address  |   |
      +------ previous fp   |   |
          | saved registers |   |
          | local variables |   |
      +-> |       ...       |   |
      |   +-----------------+   |
      |   | return address  |   |
      |   |   previous fp ------+
      |   | saved registers |
      |   | local variables |
      |   |       ...       | <-+
      |   +-----------------+   |
      |   | return address  |   |
      +------ previous fp   |   |
          | saved registers |   |
          | local variables |   |
  $fp --> |       ...       |   |
          +-----------------+   |
          | return address  |   |
          |   previous fp ------+
          | saved registers |
  $sp --> | local variables |
          +-----------------+
```

因此只要获取`fp`寄存器的值，就能够得到返回地址`fp-8`，接下来可以获取上一个栈帧`fp-16`。另外由于内核栈只有一页大小，因此只要判断`fp`不在一页内就可以终止。

```c
void
backtrace(void)
{
  uint64 fp;
  uint64 ret;
  uint64 up, down;

  printf("DEBUG: backtrace...\n");

  fp = r_fp();
  up = PGROUNDUP(fp);
  down = PGROUNDDOWN(fp);
  while (fp >= down && fp < up) {
    ret = *(uint64*)(fp - 8);
    fp = *(uint64*)(fp - 16);
    printf("%p\n", ret);
  }
}
```

## alarm

看下系统调用是按照如下步骤执行的

1. 用户态（U-mode）下执行`ecall`指令
2. 切换到S-mode，执行[`uservec`](https://github.com/mit-pdos/xv6-riscv/blob/riscv/kernel/trampoline.S#L21)
3. 保存一般寄存器到[`p->trapframe`](https://github.com/mit-pdos/xv6-riscv/blob/riscv/kernel/proc.h#L43)
4. 将页表切换到内核的页表，进入内核态
5. 执行[`usertrap`](https://github.com/mit-pdos/xv6-riscv/blob/riscv/kernel/trap.c#L37)，将[`p->trapframe->epc`](https://github.com/mit-pdos/xv6-riscv/blob/riscv/kernel/proc.h#L47)设置为中断发生处的指令地址，执行系统调用并将系统调用返回值保存在[`p->trapframe->a0`](https://github.com/mit-pdos/xv6-riscv/blob/riscv/kernel/proc.h#L58)
6. 执行[`usertrapret`](https://github.com/mit-pdos/xv6-riscv/blob/riscv/kernel/trap.c#L90)，从[`p->trapframe`](https://github.com/mit-pdos/xv6-riscv/blob/riscv/kernel/proc.h#L43)恢复中断时的寄存器的值，并将pc设置为[`p->trapframe->epc`](https://github.com/mit-pdos/xv6-riscv/blob/riscv/kernel/proc.h#L47)，从内核态回到用户态（U-mode），恢复执行

这个任务是实现两个系统调用。
- `sigalarm`是安装一个timer，只要进程在`RUNNING`状态下每达到一个`ticks`，就执行`handler`一次，当`ticks=0`时，卸载timer。
- `sigreturn`是在`handler`中执行的，每次发生`timer interrupt`，都会增加CPU正在执行的进程的`running_ticks`，因此当`running_ticks`达到安装的timer指定的`ticks`，那么`handler`就会触发一次，在`handler`中调用`sigreturn`会恢复到中断发生前的状态。

```c
int sigalarm(int ticks, void (*handler)());
int sigreturn(void);
```

当`running_ticks`等于timer指定的`ticks`时，此时`usertrapret`需要返回到timer的`handler`中，因此可以通过设置[`p->trapframe->epc`](https://github.com/mit-pdos/xv6-riscv/blob/riscv/kernel/proc.h#L47)，这样在回到用户态时会执行`handler`。

接下来在`handler`中调用`sigreturn`，会再次陷入内核态，内核需要恢复到`timer interrupt`发生时进程的状态。

在实现时有三点需要注意：

- 进入`handler`之前，我们是通过`p->trapframe`恢复所有寄存器的，并且将`p->trapframe->epc`设置为`handler`，下一步在`handler`中调用`sigreturn`，会进入`uservec`将所有寄存器保存到`p->trapframe`中，这会导致`timer interrupt`发生时的`p->trapframe`已经被重写了，因此必须在发生`timer interrupt`时进入`handler`之前，将`p->trapframe`另存一个副本，这样`sigreturn`就可以通过该副本恢复到`timer interrupt`时的状态。
- 如果`handler`执行过程中，再次触发`timer interrupt`并且满足安装的`timer`指定的ticks，需要加以限制避免再次重入`handler`中。
- 发生`timer interrupt`虽然执行了`handler`，并且`handler`中执行了`sigreturn`系统调用，此时一定会更改`a0`寄存器的值，但是期望情况是，`handler`的执行对于用户来说是透明的，因此不应该修改进程的状态，这个可以通过`sigreturn`直接返回`p->trapframe->a0`的值来避免寄存器`a0`的值被修改。

```c
// kernel/sysproc.c
uint64 
sys_sigalarm(void) 
{
  int ticks;
  uint64 handler;
  struct proc *p;

  argint(0, &ticks);
  argaddr(1, &handler);
  p = myproc();
  p->ticks = ticks;
  p->handler = handler;
  p->running_ticks = 0;
  p->inhandler = 0;
  return 0;
}

uint64 
sys_sigreturn(void) 
{
  memmove(myproc()->trapframe, myproc()->trapframe2, sizeof(struct trapframe));
  myproc()->inhandler = 0;
  return myproc()->trapframe->a0;
}

// kernel/trap.c
void
usertrap(void)
{
...
  // give up the CPU if this is a timer interrupt.
  if(which_dev == 2){
    p->lastticks += 1;
    if (p->ticks > 0 && p->running_ticks == p->ticks && p->inhandler == 0) {
      // save p->trapframe to p->trapframe2
      memmove(p->trapframe2, p->trapframe, sizeof(struct trapframe));
      // set p->trapframe->epc to p->handler
      p->trapframe->epc = p->handler;
      // acquire p->inhandler
      p->inhandler = 1;
      // reset p->lastticks
      p->running_ticks = 0;
    }
    yield();
  }
...
}
```