import logging
import os
import sys
from typing import Optional, Dict, TYPE_CHECKING

if TYPE_CHECKING:
    pass

logger = logging.Logger("assembler")

class SymbolTable(object):
    _symbol_table: Dict[str, int] = {}

    def __init__(self):
        self._symbol_table = {
            "SP": 0,
            "LCL": 1,
            "ARG": 2,
            "THIS": 3,
            "THAT": 4,
            "R0": 0,
            "R1": 1,
            "R2": 2,
            "R3": 3,
            "R4": 4,
            "R5": 5,
            "R6": 6,
            "R7": 7,
            "R8": 8,
            "R9": 9,
            "R10": 10,
            "R11": 11,
            "R12": 12,
            "R13": 13,
            "R14": 14,
            "R15": 15,
            "SCREEN": 16384,
            "KBD": 24576
        }

    def addEntry(self, symbol: str, address: int):
        if symbol in self._symbol_table:
            raise ValueError(f"Symbol {symbol} already exists in the symbol table")
        self._symbol_table[symbol] = address

    def contains(self, symbol: str) -> bool:
        return symbol in self._symbol_table

    def getAddress(self, symbol: str) -> int:
        if not symbol in self._symbol_table:
            raise ValueError(f"Symbol {symbol} not exist")
        return self._symbol_table[symbol]


A_COMMAND = 0
C_COMMAND = 1
L_COMMAND = 2 


class Command(object):
    def __init__(self, command = ""):
        self._command = command

    def commandType(self):
        assert self._command, "Call commandType() for none"
        if self._command[0] == '@':
            return A_COMMAND
        elif self._command[0] == '(':
            return L_COMMAND
        else:
            return C_COMMAND

    def symbol(self):
        command_type = self.commandType()
        assert command_type != C_COMMAND, f"Call symbol() only for A-instruction or Label but {command_type}"
        if command_type == L_COMMAND:
            return self._command[1:-1]
        else:
            return self._command[1:]
    
    def dest(self):
        assert self.commandType() == C_COMMAND, f"Call dest() only for C-instruction but {self.commandType()}"
        try:
            i = self._command.index('=')
            return self._command[0:i]
        except:
            return "null"

    def comp(self):
        assert self.commandType() == C_COMMAND, f"Call comp() only for C-instruction but {self.commandType()}"
        try:
            i = self._command.index('=')
            i += 1
        except:
            i  = 0
        try: 
            j = self._command.index(';')
        except:
            j = len(self._command)

        return self._command[i:j]

    def jump(self):
        assert self.commandType() == C_COMMAND, f"Call comp() only for C-instruction but {self.commandType()}"
        try:
            i = self._command.index(';')
            return self._command[i+1:]
        except:
            return "null"


class Parser(object):
    def __init__(self, file_name):
        self._file = open(file_name, "r")
        self._current_command = Command()
        self._line = ""

    def reset(self):
        self._file.seek(0, os.SEEK_SET)
        self._current_command = Command()
        self._line = ""

    def hasMoreCommands(self):
        for line in self._file:
            line = line[:-1]
            line = line.strip()
            if not line or line.startswith("//"):
                continue
            self._line = line
            return True
        return False

    def advance(self):
        if self._line:
            self._current_command = Command(self._line)
        else:
            if not self.hasMoreCommands():
                raise EOFError("Reach EOF")
            self._current_command = Command(self._line)
        self._line = ""

    def commandType(self):
        return self._current_command.commandType()

    def symbol(self):
        return self._current_command.symbol()

    def dest(self):
        return self._current_command.dest()

    def comp(self):
        return self._current_command.comp()

    def jump(self):
        return self._current_command.jump()

    def __iter__(self):
        return self

    def __next__(self):
        if not self.hasMoreCommands():
            raise StopIteration
        self.advance()
        return self._current_command
    
    def __del__(self):
        self._file.close() 

class Code(object):
    dest_code: Dict[str, str] = {
        "null": "000",
        "M": "001",
        "D": "010",
        "MD": "011",
        "A": "100",
        "AM": "101",
        "AD": "110",
        "AMD": "111"
    }

    comp_code: Dict[str, str] = {
        "0": "0101010",
        "1": "0111111",
        "-1": "0111010",
        "D": "0001100",
        "A": "0110000",
        "M": "1110000",
        "!D": "0001101",
        "!A": "0110001",
        "!M": "1110001",
        "-D": "0001101",
        "-A": "0110011",
        "-M": "1110011",
        "D+1": "0011111",
        "A+1": "0110111",
        "M+1": "1110111",
        "D-1": "0001110",
        "A-1": "0110010",
        "M-1": "1110010",
        "D+A": "0000010",
        "D+M": "1000010",
        "D-A": "0010011",
        "D-M": "1010011",
        "A-D": "0000111",
        "M-D": "1000111",
        "D&A": "0000000",
        "D&M": "1000000",
        "D|A": "0010101",
        "D|M": "1010101"
    }

    jump_code: Dict[str, str] = {
        "null": "000",
        "JGT": "001",
        "JEQ": "010",
        "JGE": "011",
        "JLT": "100",
        "JNE": "101",
        "JLE": "110", 
        "JMP": "111"
    }

    def __init__(self):
        pass

    def dest(self, mnemonic: str):
        ret = self.dest_code.get(mnemonic)
        if not ret:
            raise ValueError(f"Invalid dest mnemonic {mnemonic}")
        return ret

    def comp(self, mnemonic: str):
        ret = self.comp_code.get(mnemonic)
        if not ret:
            raise ValueError(f"Invalid comp mnemonic {mnemonic}")
        return ret
    
    def jump(self, mnemonic: str):
        ret = self.jump_code.get(mnemonic)
        if not ret:
            raise ValueError(f"Invalid jump mnemonic {mnemonic}")
        return ret


def main(file_name: str): 
    parser = Parser(file_name)
    symbol_table = SymbolTable()
    code = Code()

    # process label in first pass
    rom_addr = 0
    for command in parser:
        if command.commandType() == L_COMMAND:
            symbol_table.addEntry(command.symbol(), rom_addr)
        else:
            rom_addr += 1

    parser.reset()
    basename, _ = os.path.splitext(file_name)
    hack_file_name = f"{basename}.hack"
    with open(hack_file_name, "w+") as hack_file:
        ram_addr = 16
        for command in parser:
            binary = ""
            if command.commandType() == L_COMMAND:
                continue
            if command.commandType() == A_COMMAND:
                symbol = command.symbol()
                addr = 0
                if symbol.isdigit():
                    addr = int(symbol)
                else:
                    if not symbol_table.contains(symbol):
                        symbol_table.addEntry(symbol, ram_addr)
                        addr = ram_addr
                        ram_addr += 1
                    else:
                        addr = symbol_table.getAddress(symbol)
                bin_addr = bin(addr)[2:]
                binary = bin_addr.zfill(16)
            else:
                binary = "111"
                try:
                    binary = binary + code.comp(command.comp())
                    binary = binary + code.dest(command.dest())
                    binary = binary + code.jump(command.jump())
                except:
                    logger.warning(command._command)
                    raise
            hack_file.writelines([binary + "\n"])

def usage():
    print("Usage: Assembler file.asm\n")

if __name__ == "__main__":
    if len(sys.argv) < 2:
        usage()
    else:
        main(sys.argv[1])
