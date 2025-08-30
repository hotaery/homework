// This file is part of www.nand2tetris.org
// and the book "The Elements of Computing Systems"
// by Nisan and Schocken, MIT Press.
// File name: projects/04/Fill.asm

// Runs an infinite loop that listens to the keyboard input.
// When a key is pressed (any key), the program blackens the screen
// by writing 'black' in every pixel;
// the screen should remain fully black as long as the key is pressed. 
// When no key is pressed, the program clears the screen by writing
// 'white' in every pixel;
// the screen should remain fully clear as long as no key is pressed.

// row(0) to row(x-1) is black
// row(x) to row(255) is white

    @x
    M=0         // x = 0
    @reverse
    M=0         // reverse = 0
    @fillColor
    M=0         // fillColor = 0
    @lastColor
    M=0         // lastColor = 0
    @i
    M=0         // i = 0 => x map to i in RAM
    @j
    M=0         // j = 0 => paint current word
    @word
    M=0         // word = 0 => word index

(INFINITE)
    @fillColor
    M=0
    @KBD
    D=M
    @FILL
    D;JEQ
    @fillColor
    M=-1    // if KBD != 0 fillColor = -1

(FILL)
    @fillColor
    D=M
    @WHITE
    D;JEQ
    @x
    D=M
    @255
    D=D-A
    @INFINITE
    D;JGT       // if fillColor != 0 && x > 255 goto INFINITE => screen is black, do nothing
    @FILL_EXEC
    0;JMP

(WHITE)
    @x
    D=M
    @INFINITE
    D;JEQ       // if fillColor == 0 && x == 0 goto INFINITE  => screen is white, do nothing
    @x
    M=M-1       // if fillColor == 0 x = x - 1

(FILL_EXEC)
    @x
    D=M
    @R0
    M=D
    @32
    D=A
    @R1
    M=D
    @FILL_CONT
    D=A
    @R3
    M=D
    @MULT
    0;JMP

(FILL_CONT)
    @R2
    D=M
    @i
    M=D         // i = x * 32
    @fillColor
    D=M
    @lastColor
    D=D-M
    @FILL_EXEC_2
    D;JEQ
    @reverse
    M=!M        // if fillColor != lastColor reverse=!reverse

(FILL_EXEC_2)
    @reverse
    D=M
    @FILL_FORWARD  
    D;JEQ
    @32
    D=A
    @j
    M=D         // if reverse != 0 j = 32

(REVERSE_LOOP)
    @j
    MD=M-1       // j = j - 1
    @FILL_STATE
    D;JLT       // if reverse != 0 && j < 0 goto FILL_STATE 
    @SCREEN
    D=A
    @i
    D=D+M
    @j
    D=D+M
    @word
    M=D         // word = SCREEN + i + j
    @fillColor
    D=M
    @word
    A=M
    M=D         // word = fillColor
    @REVERSE_LOOP
    0;JMP

(FILL_FORWARD)
    @j          // j = 0
    M=0

(FORWARD_LOOP)
    @SCREEN
    D=A
    @i
    D=D+M
    @j
    D=D+M
    @word
    M=D         // word = SCREEN + i + j
    @fillColor
    D=M
    @word
    A=M
    M=D         // word = fillColor
    @j
    MD=M+1      // j = j + 1
    @32
    D=D-A
    @FILL_STATE
    D;JGE       // if j >= 32 goto FILL_STATE
    @FORWARD_LOOP
    0;JMP

(FILL_STATE)
    @fillColor
    D=M
    @STATE_2
    D;JEQ
    @x
    M=M+1       // if fillColor != 0 x = x + 1

(STATE_2)
    @fillColor
    D=M
    @lastColor
    M=D         // lastColor = fillColor
    @reverse
    M=!M        // reverse = !reverse
    @INFINITE
    0;JMP

// R2=R0*R1
// R3 is caller
// MULT use R0-R4
(MULT)
    @R2
    M=0     // ans = 0
    @R4
    M=1     // i = 1

(MULT_LOOP)
    @R4
    D=M
    @R1
    D=D-M
    @MULT_STOP
    D;JGT   // if i > RAM[1] goto STOP

    @R2
    D=M
    @R0
    D=D+M   
    @R2
    M=D     // ans = ans + RAM[0]

    @R4
    D=M
    D=D+1
    @R4
    M=D     // i = i + 1

    @MULT_LOOP
    0;JMP

(MULT_STOP)
    @R3
    A=M
    0;JMP

// R6 = R5 >> 1
// R7 is caller
// MOVE use R5-R10
(MOVE)
    @R6
    M=0     // R6 = 0
    @R8
    M=1     // prev = 1
    @2
    D=A
    @R9
    M=D    // next = 2
    @R10
    M=0     // i = 0

(MOVE_LOOP)
    @R10
    MD=M+1
    @15
    D=D-A
    @MOVE_STOP
    D;JGT       // if i > 15 goto MOVE_STOP
    @R5
    D=M
    @R9
    D=D&M     
    @MOVE_STATE
    D;JEQ       // if R5 & next == 0 goto MOVE_STATE
    @R6
    D=M
    @R8
    D=D|M
    @R6
    M=D     // R6 = R6 | prev

(MOVE_STATE)
    @R9
    D=M
    @R8
    M=D     // prev = next
    @R0
    M=D     // R0 = next
    @2
    D=A
    @R1
    M=D     // R1 = 2
    @MOVE_CONT
    D=A
    @R3
    M=D     // R3 = MOVE_CONT
    @MULT
    0;JMP

(MOVE_CONT)
    @R2
    D=M
    @R9
    M=D     // next = next * 2
    @MOVE_LOOP
    0;JMP

(MOVE_STOP)
    @R7
    A=M
    0;JMP
