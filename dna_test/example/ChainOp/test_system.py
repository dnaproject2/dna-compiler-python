DnaCversion = '2.0.0'
from dna.interop.System.Block import GetTransactionByIndex
from dna.interop.System.Blockchain import GetTransactionByHash
from dna.interop.System.Header import GetBlockHash
from dna.interop.System.Transaction import GetTransactionHash
from dna.interop.System.Blockchain import GetBlock


def main(Height):
    Block = GetBlock(Height)
    index = 0
    Tx = GetTransactionByIndex(Block, index)
    Txhash = GetTransactionHash(Tx)
    NewTx = GetTransactionByHash(Txhash)
    BlkHash = GetBlockHash(Block)
    print("Test finished")
