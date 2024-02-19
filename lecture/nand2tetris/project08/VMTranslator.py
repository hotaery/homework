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
            try:
                comment_index = line.index("//")
                line = line[:comment_index]
            except:
                pass
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
    }

    def __init__(self, output_file: str) -> None:
        self._file = open(output_file, "w+")
        self._input_file_name = ""
        self._function_name = ""

    def setFileName(self, file_name: str):
        self._input_file_name = file_name
        instructions = []
        self._writeInstructions(f"input file: {file_name}", instructions)

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

    def _writeInstructions(self, command: str, instructions: List[str], file = None):
        write_instructions = [f"    //{command}\n"]
        instructions = [elem + "\n" for elem in instructions]
        write_instructions = write_instructions + instructions
        if not file:
            self._file.writelines(write_instructions)
        else:
            file.writelines(write_instructions)
        
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

    def _writeStaticSegmentPush(self, index: int):
        instructions = []
        instructions.append(f"    @{self._input_file_name}.{index}")
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
        elif segment in self.direct_segment_map:
            instructions = self._writeDirectSegmentPush(segment, index)
        else:
            assert segment == "static"
            instructions = self._writeStaticSegmentPush(index)
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

    def _writeStaticSegmentPop(self, index: int):
        instructions = []
        instructions.append( "    @SP")
        instructions.append( "    AM=M-1")
        instructions.append( "    D=M")
        instructions.append(f"    @{self._input_file_name}.{index}")
        instructions.append( "    M=D")
        return instructions

    def _writePop(self, segment: str, index: int):
        if segment in self.indirect_segment_map:
            instructions = self._writeIndirectSegmentPop(segment, index)
        elif segment in self.direct_segment_map:
            instructions = self._writeDirectSegmentPop(segment, index)
        else:
            assert segment == "static"
            instructions = self._writeStaticSegmentPop(index)
        return instructions

    def writePushPop(self, command: str, segment: str, index: int):
        if command == "push":
            instructions = self._writePush(segment, index)
        else:
            instructions = self._writePop(segment, index)
        self._writeInstructions(f"{command} {segment} {index}", instructions)

    def writeInit(self):
        file_name = self._file.name
        tmp_file_name = f"{file_name}.tmp"
        logger.warning(f"write Sys.init to {tmp_file_name}")
        instructions = []
        instructions.append("    @256")
        instructions.append("    D=A")
        instructions.append("    @SP")
        instructions.append("    M=D")
        with open(tmp_file_name, "w+") as f:
            self._writeInstructions("set sp=256", instructions, f)
            self.writeCall("Sys.init", 0, 1, f)
            self._file.seek(0, os.SEEK_SET)
            for line in self._file:
                f.write(line)
        self._file.close()
        logger.warning(f"Remove {file_name}")
        os.remove(file_name)
        os.rename(tmp_file_name, file_name)
        logger.warning(f"Rename {tmp_file_name} to {file_name}")
        logger.warning(f"Reopen {file_name}")
        self._file = open(file_name, "a")

    def writeLabel(self, label: str):        
        if not self._function_name:
            label = f"{self._input_file_name}:{label}"
        else:
            label = f"{self._function_name}:{label}"
        instructions = []
        instructions.append(f"({label})")
        self._writeInstructions(f"label {label}", instructions) 

    def writeGoto(self, label: str):
        if not self._function_name:
            label = f"{self._input_file_name}:{label}"
        else:
            label = f"{self._function_name}:{label}"
        instructions = []
        instructions.append(f"    @{label}")
        instructions.append( "    0;JMP")
        self._writeInstructions(f"goto {label}", instructions)

    def writeIf(self, label: str):
        if not self._function_name:
            label = f"{self._input_file_name}:{label}"
        else:
            label = f"{self._function_name}:{label}"
        instructions = []
        instructions.append( "    @SP")
        instructions.append( "    AM=M-1")
        instructions.append( "    D=M")
        instructions.append(f"    @{label}")
        instructions.append( "    D;JNE")
        self._writeInstructions(f"if-goto {label}", instructions)

    def _pushToStack(self, instructions: List[str]):
        instructions.append("    @SP")
        instructions.append("    A=M")
        instructions.append("    M=D")
        instructions.append("    @SP")
        instructions.append("    M=M+1")

    def writeCall(self, function_name: str, num_args: int, lineno: int, file = None):
        instructions = []
        # push return-address
        instructions.append(f"    @{self._input_file_name}:{lineno}:{function_name}")
        instructions.append( "    D=A")
        self._pushToStack(instructions)
        for i in ["LCL", "ARG", "THIS", "THAT"]:
            instructions.append(f"    @{i}")
            instructions.append( "    D=M")
            self._pushToStack(instructions)
        instructions.append( "    @5")
        instructions.append( "    D=A")
        instructions.append(f"    @{num_args}")
        instructions.append( "    D=D+A")
        instructions.append( "    @SP")
        instructions.append( "    A=M")
        instructions.append( "    D=A-D")
        instructions.append( "    @ARG")
        instructions.append( "    M=D")         # ARG=SP-(n+5)
        instructions.append( "    @SP")
        instructions.append( "    D=M")
        instructions.append( "    @LCL")
        instructions.append( "    M=D")         # LCL=SP
        instructions.append(f"    @{function_name}")
        instructions.append( "    0;JMP")       # goto function_name
        instructions.append(f"({self._input_file_name}:{lineno}:{function_name})")   # return label
        self._writeInstructions(f"call {function_name} {num_args}", instructions, file)

    def writeReturn(self):
        instructions = []
        instructions.append( "    @SP")
        instructions.append( "    A=M-1")
        instructions.append( "    D=M")    # stack top element
        instructions.append( "    @R13")
        instructions.append( "    M=D")    # R13=RETURN VALUE
        instructions.append( "    @ARG")
        instructions.append( "    D=M")
        instructions.append( "    @R14")
        instructions.append( "    M=D")    # R14=ARG
        instructions.append( "    @LCL")
        instructions.append( "    D=M")
        instructions.append( "    @SP")
        instructions.append( "    M=D")    # SP=LCL
        for i in ["THAT", "THIS", "ARG", "LCL", "R15"]:  # set THAT THIS ARG LCL R15=return-address
            instructions.append( "    @SP")
            instructions.append( "    M=M-1")
            instructions.append( "    @SP")
            instructions.append( "    A=M")
            instructions.append( "    D=M")
            instructions.append(f"    @{i}")
            instructions.append( "    M=D")
        instructions.append( "    @R14")
        instructions.append( "    D=M")
        instructions.append( "    @SP")
        instructions.append( "    M=D")
        instructions.append( "    @R13")
        instructions.append( "    D=M")
        instructions.append( "    @SP")
        instructions.append( "    A=M")
        instructions.append( "    M=D")
        instructions.append( "    @SP")
        instructions.append( "    M=M+1")
        instructions.append( "    @R15")
        instructions.append( "    A=M")
        instructions.append( "    0;JMP")
        self._writeInstructions("return", instructions)

    def writeFunction(self, function_name: str, num_locals: int):
        self._function_name = function_name
        if function_name == "Sys.init":
            self.writeInit()
        instructions = []
        instructions.append(f"({function_name})")
        for i in range(num_locals):
            instructions.append("    @SP")
            instructions.append("    A=M")
            instructions.append("    M=0")
            instructions.append("    @SP")
            instructions.append("    M=M+1")
        self._writeInstructions(f"function {function_name} {num_locals}", instructions)

    def close(self):
        if self._file.closed:
            self._file.flush()
            self._file.close()

def main(file_or_directory: str):
    if file_or_directory.endswith(os.path.sep):
        file_or_directory = file_or_directory[:-1]
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
    if os.path.isfile(file_or_directory):
        output_file_name = os.path.join(parent_dir, output_file_name)
    else:
        output_file_name = os.path.join(file_or_directory, output_file_name)
    logger.warning(f"Translate {vm_file_list} to {output_file_name}")
    code_writer = CodeWriter(output_file_name)
    for file_name in vm_file_list:
        base_name = os.path.splitext(os.path.basename(file_name))[0]
        logger.warning(f"Process {base_name}")
        code_writer.setFileName(base_name)
        parser = Parser(file_name)
        for command in parser:
            type = command.commandType()
            if type == C_ARITHMETIC:
                code_writer.writeArithmetic(command.name(), command.lineno())
            elif type == C_PUSH or type == C_POP:
                code_writer.writePushPop(command.name(), command.arg1(), command.arg2())
            elif type == C_GOTO:
                code_writer.writeGoto(command.arg1())
            elif type == C_IF:
                code_writer.writeIf(command.arg1())
            elif type == C_LABEL:
                code_writer.writeLabel(command.arg1())
            elif type == C_CALL:
                code_writer.writeCall(command.arg1(), command.arg2(), command.lineno())
            elif type == C_RETURN:
                code_writer.writeReturn()
            elif type == C_FUNCTION:
                code_writer.writeFunction(command.arg1(), command.arg2())
            else:
                raise NotImplementedError(f"{command.name()} {type}")
    code_writer.close()

def usage():
    print("Usage: VMTranslator file.vm or directory name\n")

if __name__ == "__main__":
    if len(sys.argv) != 2:
        usage()
    else:
        main(sys.argv[1])
