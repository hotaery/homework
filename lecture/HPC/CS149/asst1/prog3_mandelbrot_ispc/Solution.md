# Solution

## Q1

由于使用avx2指令集，每次可以处理8个单进度浮点数，因此理论上使用ISPC可以获得8倍的性能提升。然而，实际上每次8个单浮点数的运算是受限制于最复杂的那个元素的迭代次数，因此整体提升不一定能够到8倍

```
[mandelbrot serial]:            [121.302] ms
Wrote image file mandelbrot-serial.ppm
[mandelbrot ispc]:              [21.396] ms
Wrote image file mandelbrot-ispc.ppm
                                (5.67x speedup from ISPC)
```

**VIEW=2**的结果

```
[mandelbrot serial]:            [73.839] ms
Wrote image file mandelbrot-serial.ppm
[mandelbrot ispc]:              [15.024] ms
Wrote image file mandelbrot-ispc.ppm
                                (4.91x speedup from ISPC)
```

## Q2
```
[mandelbrot serial]:            [121.347] ms
Wrote image file mandelbrot-serial.ppm
[mandelbrot ispc]:              [21.315] ms
Wrote image file mandelbrot-ispc.ppm
[mandelbrot multicore ispc]:    [8.872] ms
Wrote image file mandelbrot-task-ispc.ppm
                                (5.69x speedup from ISPC)
                                (13.68x speedup from task ISPC)
```

## Q3
随着task数量增长，性能提升逐渐趋向稳定，甚至在task数量过多时性能下降。我的电脑是24核心，选择50个task时性能提升最大，继续增大反而没有收益有限。这主要是为了负载均衡，过少的task可能无法充分利用所有核心，过多的task由于硬件限制，并发有限。

|task ISPC|speedup|
|-|-|
|1|5.74x|
|2|13.68x|
|4|14.25x|
|8|23.48x|
|16|40.77x|
|20|46.15x|
|40|64.18x|
|50|70.69x|
|80|69.56x|
|100|68.52x|
|200|57.31x|
|400|24.97x|
|800|13.17x|

## Q4
- Thread (Create/Join): 是操作系统的原生抽象。创建线程涉及内核调用、独立的栈分配、寄存器状态保存等，开销昂贵。

- Task (Launch/Sync): 是应用层（ISPC 运行时）的抽象。它只是一个推入队列的函数指针和参数集合，开销极低。

ISPC的task执行更像是一个线程池，生成的task被推入一个队列，由固定数量的工作线程（通常等于CPU核心数）来执行。这种方式避免了频繁创建和销毁线程的开销，同时也能更好地利用CPU资源，实现更高效的并行计算。