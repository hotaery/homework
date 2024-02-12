// RAM[2] = RAM[0] * RAM[1]
// 
//  ans = 0
//  num = RAM[0]
//  n = RAM[1]
//  i = 1 
// LOOP:
//  if i > n goto STOP
//  ans = ans + num
//  i = i + 1
//  goto LOOP
// STOP:
//  RAM[2] = ans

    @ans
    M=0     // ans = 0

    @i
    M=1     // i = 1

(LOOP)
    @i
    D=M
    @R1
    D=D-M
    @STOP
    D;JGT   // if i > RAM[1] goto STOP

    @ans
    D=M
    @R0
    D=D+M   
    @ans
    M=D     // ans = ans + RAM[0]

    @i
    D=M
    D=D+1
    @i
    M=D     // i = i + 1

    @LOOP
    0;JMP

(STOP)
    @ans
    D=M
    @R2
    M=D     // RAM[2] = ans

(END)
    @END
    0;JMP



