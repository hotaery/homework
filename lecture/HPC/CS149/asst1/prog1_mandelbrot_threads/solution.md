# Solution

## Q1 & Q2
|线程数|耗时(ms)|加速比|
|---|---|---|
|1|242.027|-|
|2|127.463|1.90|
|3|152.949|1.58|
|4|104.374|2.33|
|5|102.339|2.38|
|6|78.758|3.09|
|7|75.204|3.23|
|8|64.118|3.79|

性能并能没有随着线程数增加而线性提升，只有当线程数为2的时候，性能提升接近2倍，之后的性能提升并不明显，甚至在线程数为3时还发生了性能下降。接着下面是VIEW=2的结果，尽管不是线性提升，但是随着线程数的增加，性能是不断提升的。

|线程数|耗时(ms)|加速比|
|---|---|---|
|1|143.223|-|
|2|87.102|1.64|
|3|65.787|2.19|
|4|55.462|2.59|
|5|48.858|2.92|
|6|42.613|3.41|
|7|39.033|3.70|
|8|35.285|4.01|

使用多线程生成ppm图片，是根据行来划分任务的，每个线程负责生成图像的一部分行，在VIEW=1以及两个线程时，由于图像是y轴对称的，因此每个线程执行的任务规模大致相同，性能能够提升接近两倍。在VIEW=2时，由于VIEW=2的图像是放大66倍的局部，在分布上（白色区域）没有明显不均衡，而白色区域的迭代次数较多，因此在VIEW=2时，随着线程数的增加，每个线程执行的任务规模也比较均衡，性能是总体提升的。

## Q3

为了验证上述猜想，计算每个线程的耗时

```c
double startTime = CycleTimer::currentSeconds();
int rowsPerThread = args->height / args->numThreads;
int startRow = args->threadId * rowsPerThread;
int numRows = (args->threadId == args->numThreads - 1) ? (args->height - startRow) : rowsPerThread;
mandelbrotSerial(args->x0, args->y0, args->x1, args->y1,
                 args->width, args->height,
                 startRow, numRows,
                 args->maxIterations,
                 args->output);
double endTime = CycleTimer::currentSeconds();
args->time = endTime - startTime;
```

|线程数|最短耗时(ms)|最短线程id|最长耗时(ms)|最长线程id|
|---|---|---|---|---|
|2|126.873|0|129.203|1|
|3|50.082|0|153.685|1|
|4|24.157|0|103.478|2|
|5|10.992|0|103.323|2|
|6|6.855|0|80.167|2|
|7|5.168|0|76.609|3|
|8|4.054|0|64.451|4|

## Q4

从Q3的结果可以看出，主要原因是因为线程的计算负载不均衡导致整体的性能提升并不是线性的，而是有计算量最大的线程决定的，为了避免这个问题，可以将任务划分的更细粒度一些，将图像按照chunk(例如16行)来划分，线程按照chunk来获取任务。加入有n个线程，那么前n个chunk每个线程执行1个，接着再执行接着的n个chunk：

```c
for (int i = 0; i < args->height; i += CHUNK_ROWS * args->numThreads) {
    int startRow = i + args->threadId * CHUNK_ROWS;
    if (startRow >= args->height) {
        break;
    }
    int numRows = CHUNK_ROWS;
    if (startRow + numRows > args->height) {
        numRows = args->height - startRow;
    }
    mandelbrotSerial(args->x0, args->y0, args->x1, args->y1,
                     args->width, args->height,
                     startRow, numRows,
                     args->maxIterations,
                     args->output);
}
```

使用这种方式划分任务，能够使得每个线程的计算负载更加均衡，从而提升性能。

|线程数|耗时(ms)|加速比|
|---|---|---|
|1|247.455|-|
|2|127.073|1.94|
|3|86.873|2.84|
|4|66.118|3.73|
|5|53.678|4.60|
|6|45.678|5.41|
|7|39.456|6.27|
|8|34.567|7.15|