from jack_token import Token, Keyword, TokenType
from typing import Dict, Tuple
import bidict


class JackTokenizer(object):

    keyword_map = bidict.bidict({
        "class"         : Keyword.CLASS,    
        "constructor"   : Keyword.CONSTRUCTOR,
        "function"      : Keyword.FUNCTION, 
        "method"        : Keyword.METHOD,
        "field"         : Keyword.FIELD, 
        "static"        : Keyword.STATIC,
        "var"           : Keyword.VAR, 
        "int"           : Keyword.INT, 
        "char"          : Keyword.CHAR,
        "boolean"       : Keyword.BOOLEARN, 
        "void"          : Keyword.VOID,
        "true"          : Keyword.TRUE, 
        "false"         : Keyword.FALSE,
        "null"          : Keyword.NULL, 
        "this"          : Keyword.THIS, 
        "let"           : Keyword.LET,
        "do"            : Keyword.DO, 
        "if"            : Keyword.IF, 
        "else"          : Keyword.ELSE,
        "while"         : Keyword.WHILE, 
        "return"        : Keyword.RETURN
    })

    token_type_map: Dict[TokenType, str] = {
        TokenType.KEYWORD:      "keyword",
        TokenType.IDENTIFIER:   "identifier",
        TokenType.INT_CONST:    "integerConstant",
        TokenType.STRING_CONST: "stringConstant",
        TokenType.SYMBOL:       "symbol"
    }

    token_value_map: Dict[str, str] = {
        "<": "&lt;",
        ">": "&gt;",
        '"': "quot;",
        "&": "&amp;"
    }

    symbol_list = ["{", "}", "(", ")", 
                   "[", "]", ".", ",", 
                   ";", "+", "-", "*", 
                   "/", "&", "|", "<", 
                   ">", "=", "~"]

    def __init__(self, file_name: str):
        self._file = open(file_name, "r")
        self._token_queue = []

    def __iter__(self):
        return self 
    
    def _validateIdentifier(self, identifier: str):
        if identifier[0] >= '0' and identifier[0] <= '9':
            return False
        for c in identifier:
            if c.isalnum() or c == '_':
                continue
            return False
        return True
    
    def _split(self, content: str):
        items = []
        start = 0
        quote = False
        for i in range(len(content)):
            if content[i] == '"':
                if quote:
                    assert content[start] == '"'
                    items.append(content[start:i+1])
                    start = i + 1
                elif i - start > 0:
                    items.append(content[start, i]);
                    start = i
                quote = not quote
            elif content[i].isspace() and not quote:
                if i - start > 0:
                    items.append(content[start:i])
                start = i + 1
            elif content[i] in self.symbol_list and not quote:
                if i - start > 0:
                    items.append(content[start:i])
                items.append(content[i])
                start = i + 1
            
        return items

    def _generateToken(self, content: str):
        items = self._split(content) 
        for item in items:
            keyword_ = self.keyword_map.get(item)
            if keyword_:
                token = Token(TokenType.KEYWORD, keyword_)
            elif item.isdigit():
                token = Token(TokenType.INT_CONST, int(item))
            elif item in self.symbol_list:
                token = Token(TokenType.SYMBOL, item)
            elif item[0] == '"':
                token = Token(TokenType.STRING_CONST, item[1:-1])
            else:
                assert self._validateIdentifier(item), f"Invalid syntax {item}"
                token = Token(TokenType.IDENTIFIER, item)
             
            self._token_queue.append(token)

    def __next__(self) -> Token:
        if self._token_queue:
            token = self._token_queue[0]
            self._token_queue = self._token_queue[1:]
            return token
        opening_comment = False 
        for line in self._file:
            line = line.strip()
            if opening_comment:
                try:
                    idx = line.index("*/")
                    line = line[idx+2:]
                    opening_comment = False
                except:
                    # in comment
                    continue
            else:
                try:
                    idx1 = line.index("/*")
                    opening_comment = True
                except:
                    idx1 = -1
                if idx1 != -1:
                    try:
                        idx2 = line.index("*/")
                        line = line[:idx1] + line[idx2+2:]
                        opening_comment = False
                    except:
                        line = line[:idx1]
                        idx2 = -1
                try:
                    idx = line.index("//")
                    line = line[:idx]
                except:
                    pass
                    
            if line.endswith("\n"):
                line = line[-1]
            if not line:
                continue
            self._generateToken(line)
            if self._token_queue:
                token = self._token_queue[0]
                self._token_queue = self._token_queue[1:]
                return token
        raise StopIteration

    
    @staticmethod
    def tokenToString(token: Token) -> Tuple[str, str]: 
        token_type = token.token_type
        token_value = token.value
        if token_type == TokenType.KEYWORD:
            token_value = JackTokenizer.keyword_map.inverse.get(token_value)
        elif isinstance(token_value, int):
            token_value = str(token_value)
        return JackTokenizer.token_type_map[token_type], token_value


def _test():
    from xml.etree import ElementTree as ET
    import sys
    print(sys.argv[1], sys.argv[2])
    tokenizer = JackTokenizer(sys.argv[1])
    root = ET.Element("tokens")
    for token in tokenizer:
        token_type, token_value = JackTokenizer.tokenToString(token)
        child = ET.Element(token_type)
        child.text = token_value
        root.append(child)
    tree = ET.ElementTree(root)
    ET.indent(tree)
    tree.write(sys.argv[2])


if __name__ == "__main__":
    _test()
