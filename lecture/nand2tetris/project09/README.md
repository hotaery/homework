# 数独

## 游戏规则

数独包含一个大小9*9的棋盘，每个格子都包含一个1到9的数组，游戏目标是使用数字将棋盘填满，并且满足
- 每一行都包含1到9的所有数字
- 每一列都包含1到9的所有数字
- 将棋盘分成9个不重叠的3*3的小棋盘，每个小棋盘都包含1到9的所有数字

## 详细设计

棋盘每个单元格大小为`28*28`像素，每个单元格包含宽度为2个像素的边界，棋盘四周的边界宽度也为2个像素。通过设计棋盘类`Borad`和数独类`Sudoku`，`Borad`负责棋盘的显示，而`Sudoku`是游戏逻辑的实现，包括和键盘的交互，更新`Borad`的状态。

`Borad`API设计如下

|API|参数|描述|备注|
|-|-|-|-|
|`constructor`|shiftRight: int|shiftRight表示棋盘向右偏移的像素数||
|`drawBoundary`||绘制棋盘边界||
|`drawCell`|x: int</br>y: int|绘制第`y`行第`x`列的单元格，x和y均在[0, 8]||
|`heightlightCell`|x: int</br>y: int|高亮显示第`y`行第`x`列的单元格，x和y均在[0,8]||
|`draw`||绘制整个棋盘|只会调用一次|
|`drawCurrentCell`||绘制当前所在单元格||
|`set`|x: int</br>y: int</br>val: int|设置单元格的值，x和y均在[0-8]||
|`get`|x: int</br>y: int|获取单元格的值，x和y均在[0-8]||
|`setCurrent`|val: int|设置当前所在单元格的值||
|`getCurrent`||获取当前所在单元格的值||
|`getCurrentX`||获取当前所在单元格的列数||
|`getCurrentY`||获取当前所在单元格的行数||
|`getOldCurrentX`||移动前所在单元格的列数||
|`getOldCurrentY`||移动前所在单元格的行数||
|`moveUp`||向上移动当前所在单元格||
|`moveDown`||向下移动当前所在单元格||
|`moveLeft`||向左移动当前所在单元格||
|`moveRight`||向右移动当前所在单元格||
|`allFilled`||棋盘是否已经被填满||
|`dispose`||释放资源||
|`drawCellBoundary`|x: int</br>y: int|像素级别绘制单元格的边界，x和y为单元格左上角像素的坐标，x在[0-511]，y在[0-256]|静态方法|
|`drawCurrentCellBoundary`|x: int</br>y: int|像素级别显示当前所在单元格的边界，高亮显示，x在[0-511]，y在[0-256]|静态方法|
|`drawOne`</br>`drawTwo`</br>`drawThree`</br>`drawFour`</br>`drawFive`</br>`drawSix`</br>`drawSeven`</br>`drawEight`</br>`drawNine`|x: int</br>y: int|像素级别绘制1-9数字|静态方法||
|`clearCell`|x: int</br>y: int|像素级别清空当前单元格的数字，只留下边界，用于更新单元格数值||

`Sudoku`API如下

|API|参数|描述|备注|
|-|-|-|-|
|constructor|shift: int|初始化棋盘||
|check|val: int|检查是否能够在当前所在单元格设置val||
|run||运行数独逻辑，直到主动退出或者游戏胜利||
|dispose||清理资源||
|initBoard|board: Board|初始化棋盘|静态方法|

## 操作

- `ESC`: 退出游戏
- `up`: 向上移动
- `down`: 向下移动
- `left`: 向左移动
- `right`: 向右移动
- `backspace`: 删除当前单元格数字
- `0`: 删除当前单元格数字
- `1-9`: 设置当前单元格数字

## TODO

当前只有一种形式的棋盘，需要通过添加一些随机性来初始化棋盘。

另外可以通过初始化时填充的数字个数来区分不同的难度。

## 效果

<video width="630" height="300" src="https://github.com/hotaery/homework/blob/master/lecture/nand2tetris/project09/Sudoku/imp.mp4"></video>

