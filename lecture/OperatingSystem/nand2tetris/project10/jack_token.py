from typing import Union
from enum import Enum, auto


class TokenType(Enum):
    KEYWORD         = auto()
    SYMBOL          = auto()
    IDENTIFIER      = auto()
    INT_CONST       = auto()
    STRING_CONST    = auto()

class Keyword(Enum):
    CLASS           = auto()
    METHOD          = auto()
    FUNCTION        = auto()
    CONSTRUCTOR     = auto()
    INT             = auto()
    BOOLEARN        = auto()
    CHAR            = auto()
    VOID            = auto()
    VAR             = auto()
    STATIC          = auto()
    FIELD           = auto()
    LET             = auto()
    DO              = auto()
    IF              = auto()
    ELSE            = auto()
    WHILE           = auto()
    RETURN          = auto()
    TRUE            = auto()
    FALSE           = auto()
    NULL            = auto()
    THIS            = auto()


class Token(object):
    def __init__(self, token_type: TokenType, value: Union[Keyword, int, str]):
        self._token_type = token_type
        self._value = value

    @property
    def token_type(self):
        return self._token_type
    
    @property
    def value(self):
        return self._value

    def __str__(self):
        return self._token_type.name


def _test():
    token = Token(TokenType.KEYWORD, Keyword.CLASS)
    print(token, token.value.name)


if __name__ == "__main__":
    _test()
