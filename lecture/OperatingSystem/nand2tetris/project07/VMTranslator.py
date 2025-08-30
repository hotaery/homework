import functools
import logging
import os
import sys
from typing import Dict, List, TYPE_CHECKING

if TYPE_CHECKING:
    pass


logger = logging.Logger("VMTranslator")

C_ARITHMETIC    = 0
C_PUSH          = 1
C_POP           = 2
C_LABEL         = 3
C_GOTO          = 4
C_IF            = 5
C_FUNCTION      = 6
C_RETURN        = 7
C_CALL          = 8


def _validateCommand(func):
    @functools.wraps(func)
    def wrapper(self, *args, **kwargs):
        if not self._command or not self._command_items:
            raise ValueError(f"Not allow to call {func.__name__} for none command")
        if not self._command_items[0] in self.command_argument_number:
            raise ValueError(f"Invalid command {self._command}")
        n = self.command_argument_number.get(self._command_items[0])
        if n + 1 != len(self._command_items):
            raise ValueError(f"Command {self._command_items[0]} MUST have {n} argument(s) but {len(self._command_items) - 1}")
        return func(self, *args, **kwargs)
    return wrapper


class Command(object):

    command_argument_number: Dict[str, int] = {
        "add": 0, "sub": 0, "neg": 0, "eq": 0, "gt": 0, 
        "lt": 0, "and": 0, "or": 0, "not": 0,
        "push": 2, "pop": 2, "label": 1, "goto": 1, 
        "if-goto": 1, "function": 2, "call": 2, "return": 0
    }

    command_type: Dict[str, int] = {
        "add": C_ARITHMETIC, "sub": C_ARITHMETIC, "neg": C_ARITHMETIC,
        "eq": C_ARITHMETIC, "gt": C_ARITHMETIC, "lt": C_ARITHMETIC, 
        "and": C_ARITHMETIC, "or": C_ARITHMETIC, "not": C_ARITHMETIC, 
        "push": C_PUSH, "pop": C_POP, "label": C_LABEL, "goto": C_GOTO, 
        "if-goto": C_IF, "function": C_FUNCTION, "call": C_CALL, "return": C_RETURN
    } 

    binary_command: List[str] = [
        "add", "sub", "eq", "gt", "lt", "and", "or", 
    ]

    def __init__(self, lineno: int = 0, command = ""):
        command = command.strip()
        self._command = command
        self._command_items = command.split()
        self._lineno = lineno

    @staticmethod
    def isBinaryCommand(command: str):
        return command in Command.binary_command
    
    @_validateCommand
    def commandType(self):
        return self.command_type.get(self._command_items[0])
    
    @_validateCommand
    def name(self):
        return self._command_items[0]

    def arg1(self):
        command_type = self.commandType()
        assert command_type != C_RETURN, f"Return command doesn't have arg1"
        if len(self._command_items) == 1:
            return self._command_items[0]
        else:
            return self._command_items[1]

    def arg2(self):
        command_type = self.commandType()
        assert command_type == C_PUSH or \
               command_type == C_POP or \
               command_type == C_FUNCTION or \
               command_type == C_CALL, f"Command {command_type} doesn't allow to call arg2()"
        try:
            return int(self._command_items[2])
        except:
            raise ValueError(f"Invalid command {self._command}")

    def lineno(self):
        return self._lineno


class Parser(object):
    def __init__(self, file_name: str):
        self._file = open(file_name, "r")
        self._current_command = Command()
        self._line = ""
        self._current_lineno = 0

    def __del__(self):
        self._file.close()

    def __iter__(self):
        return self
    
    def __next__(self) -> Command:
        if not self.hasMoreCommands():
            raise StopIteration
        self.advance()
        return self._current_command

    def hasMoreCommands(self) -> bool:
        for line in self._file:
            # remove \n
            line = line[:-1] 
            line = line.strip()
            # ignore comment and white space
            if not line or line.startswith("//"):
                continue
            self._line = line
            self._current_lineno += 1
            return True
        return False

    def advance(self):
        if self._line:
            self._current_command = Command(self._current_lineno, self._line)
        else:
            if not self.hasMoreCommands():
                raise EOFError("Reach EOF")
            self._current_command = Command(self._current_lineno, self._line)    
        self._line = ""

    def commandType(self):
        return self._current_command.commandType()

    def arg1(self):
        return self._current_command.arg1()

    def arg2(self):
        return self._current_command.arg2()


class CodeWriter(object):
    vm_command_map_assemble_code: Dict[str, str] = {
        "add":  "    D=D+M",
        "sub":  "    D=M-D",
        "neg":  "    D=-D",
        "and":  "    D=D&M",
        "or":   "    D=D|M",
        "not":  "    D=!D",
        "eq": """    D=M-D
    @{}.{}.1
    D;JNE
    D=-1
    @{}.{}.2
    0;JMP
({}.{}.1)
    D=0
({}.{}.2)""",

        "gt": """    D=M-D
    @{}.{}.1
    D;JLE
    D=-1
    @{}.{}.2
    0;JMP
({}.{}.1)
    D=0
({}.{}.2)""",

        "lt": """    D=M-D
    @{}.{}.1
    D;JGE
    D=-1
    @{}.{}.2
    0;JMP
({}.{}.1)
    D=0
({}.{}.2)"""
    }

    indirect_segment_map: Dict[str, str] = {
        "local":    "LCL",
        "argument": "ARG",
        "this":     "THIS",
        "that":     "THAT",
    }

    direct_segment_map: Dict[str, int] = {
        "temp":    5,
        "pointer": 3,
        "static":  16
    }

    def __init__(self, output_file: str) -> None:
        self._file = open(output_file, "w+")
        self._input_file_name = ""
        self.hasHeader = False
    
    def __del__(self):
        self.close() 

    def setFileName(self, file_name: str):
        self._input_file_name = file_name

    def writeArithmetic(self, command: str, lineno: int):
        is_binary = Command.isBinaryCommand(command)
        instructions: List[str] = []
        # get y => D=y
        instructions.append("    @SP")
        instructions.append("    M=M-1")
        instructions.append("    @SP")
        instructions.append("    A=M")
        instructions.append("    D=M")
        if is_binary:
            # get x => M=x
            instructions.append("    @SP")
            instructions.append("    M=M-1")
            instructions.append("    @SP")
            instructions.append("    A=M")
        map_instruction = self.vm_command_map_assemble_code.get(command)
        assert map_instruction, f"Invalid command {command}"
        if self._isConditionalCommand(command):
            map_instruction = map_instruction.format(self._input_file_name, 
                                                    lineno, self._input_file_name, lineno,
                                                    self._input_file_name, lineno, 
                                                    self._input_file_name, lineno)
        instructions.append(map_instruction)
        # push to stack 
        instructions.append("    @SP")
        instructions.append("    A=M")
        instructions.append("    M=D")
        instructions.append("    @SP")
        instructions.append("    M=M+1")
        self._writeInstructions(command, instructions)

    def _writeInstructions(self, command: str, instructions: List[str]):
        write_instructions = [f"    //{command}\n"]
        instructions = [elem + "\n" for elem in instructions]
        write_instructions = write_instructions + instructions
        self._file.writelines(write_instructions)
        
    def _isConditionalCommand(self, command: str):
        return command == "eq" or command == "gt" or command == "lt"

    def _writeConstantPush(self, index: int):
        instructions = []
        instructions.append(f"    @{index}")
        instructions.append( "    D=A")
        instructions.append( "    @SP")
        instructions.append( "    A=M")
        instructions.append( "    M=D")
        return instructions

    def _writeIndirectSegmentPush(self, segment: str, index: int):
        instructions = []
        symbol = self.indirect_segment_map.get(segment)
        instructions.append(f"    @{symbol}")
        instructions.append( "    D=M")
        instructions.append(f"    @{index}")
        instructions.append( "    A=D+A")
        instructions.append( "    D=M")
        instructions.append( "    @SP")
        instructions.append( "    A=M")
        instructions.append( "    M=D")
        return instructions

    def _writeDirectSegmentPush(self, segment: str, index: int):
        instructions = []
        base = self.direct_segment_map.get(segment)
        addr = base + index
        instructions.append(f"    @{addr}")
        instructions.append( "    D=M")
        instructions.append( "    @SP")
        instructions.append( "    A=M")
        instructions.append( "    M=D")
        return instructions

    def _writePush(self, segment: str, index: int):
        if segment == "constant":
            instructions = self._writeConstantPush(index)
        elif segment in self.indirect_segment_map:
            instructions = self._writeIndirectSegmentPush(segment, index)
        else:
            assert segment in self.direct_segment_map
            instructions = self._writeDirectSegmentPush(segment, index)
        instructions.append("\t@SP")
        instructions.append("\tM=M+1")
        return instructions        

    def _writeIndirectSegmentPop(self, segment: str, index: int):
        instructions = []
        symbol = self.indirect_segment_map.get(segment)
        instructions.append( "    @SP")
        instructions.append( "    M=M-1")
        instructions.append( "    @SP")
        instructions.append( "    A=M")
        instructions.append( "    D=M")
        instructions.append( "    @R13")
        instructions.append( "    M=D")
        instructions.append(f"    @{symbol}")
        instructions.append( "    D=M")
        instructions.append(f"    @{index}")
        instructions.append( "    D=D+A")
        instructions.append( "    @R14")
        instructions.append( "    M=D")
        instructions.append( "    @R13")
        instructions.append( "    D=M")
        instructions.append( "    @R14")
        instructions.append( "    A=M")
        instructions.append( "    M=D")
        return instructions
        
    def _writeDirectSegmentPop(self, segment: str, index: int):
        instructions = []
        base = self.direct_segment_map.get(segment)
        addr = base + index
        instructions.append( "    @SP")
        instructions.append( "    M=M-1")
        instructions.append( "    @SP")
        instructions.append( "    A=M")
        instructions.append( "    D=M")
        instructions.append(f"    @{addr}")
        instructions.append( "    M=D")
        return instructions

    def _writePop(self, segment: str, index: int):
        if segment in self.indirect_segment_map:
            instructions = self._writeIndirectSegmentPop(segment, index)
        else:
            assert segment in self.direct_segment_map
            instructions = self._writeDirectSegmentPop(segment, index)
        return instructions

    def writePushPop(self, command: str, segment: str, index: int):
        if command == "push":
            instructions = self._writePush(segment, index)
        else:
            instructions = self._writePop(segment, index)
        self._writeInstructions(f"{command} {segment} {index}", instructions)

    def close(self):
        if self._file.closed:
            self._file.flush()
            self._file.close()

def main(file_or_directory: str):
    vm_file_list: List[str] = []
    parent_dir = os.path.dirname(file_or_directory)
    if os.path.isdir(file_or_directory):
        children = os.listdir(file_or_directory)
        for child in children:
            child_path = os.path.join(file_or_directory, child)
            if os.path.isfile(child_path) and os.path.splitext(child)[1] == ".vm":
                vm_file_list.append(child_path)
    else:
        vm_file_list.append(file_or_directory)
    base_name = os.path.basename(file_or_directory)
    output_file_name = f"{os.path.splitext(base_name)[0]}.asm"
    output_file_name = os.path.join(parent_dir, output_file_name)
    logger.warning(f"Translate {vm_file_list} to {output_file_name}")
    code_writer = CodeWriter(output_file_name)
    for file_name in vm_file_list:
        base_name = os.path.splitext(os.path.basename(file_name))[0]
        code_writer.setFileName(base_name)
        parser = Parser(file_name)
        for command in parser:
            type = command.commandType()
            if type == C_ARITHMETIC:
                code_writer.writeArithmetic(command.name(), command.lineno())
            elif type == C_PUSH or type == C_POP:
                code_writer.writePushPop(command.name(), command.arg1(), command.arg2())
            else:
                raise NotImplementedError
    code_writer.close()

def usage():
    print("Usage: VMTranslator file.vm or directory name\n")

if __name__ == "__main__":
    if len(sys.argv) != 2:
        usage()
    else:
        main(sys.argv[1])
