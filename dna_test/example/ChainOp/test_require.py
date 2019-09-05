DnaCversion = '2.0.0'
from dna.lib.require import Require, RequireIsAddress, RequireWitness


def Main(operation, args):
    if operation == "requireTest":
        Require(len(args) == 1)
        return requireTest(args[0])
    return False


def requireTest(address):
    RequireIsAddress(address)
    RequireWitness(address)
    return True
