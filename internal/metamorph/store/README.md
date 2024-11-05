# RawStore

We want to store bitcoin transactions in efficient block storage with as little overhead as possible.


## Possible option 1
We could store each transaction in a folder using the txid as the filename.  The problem with this is that filesystems become very slow when there are thousands of files in a folder.  Also, bitcoin transactions are often around 100 bytes in size but modern filesystems use block sizes of 4kB or more, which means that a 100 byte transaction actually uses 4096 bytes of disk space.

## Possible option 2
We could store transactions over a range of folders based on the first characters of the txid.  This would solve the filesystem performance issue but is still not efficient for storage.

## Option 3
This solution is inspired by the way bitcoind stores its blocks and transactions.

We store each transaction sequentially in a file.  When the next transaction will cause the file to be more than 128MB, we open a new file.  Files are named with a number in the filename which is incremented each time a new file is opened.  After writing a transaction to a file, we store the filename that was used and offset in a database.

## Deletion
From time-to-time we want to delete transactions from disk when we don't need them any more.  However, it is possible that a transaction to be deleted is sitting in the same file as a transaction that we want to keep.  Therefore, we need a strategy for this.

The plan is to delete all transactions after they have had 100 confirmations (~ 24 hours).  We do want to keep unconfirmed transactions longer, let's say 1 week.  When we want to delete an individual file, we will delete its row in the database but we need another approach for the actual on-disk storage.

By choosing a filename in the form: ```transactions_yyyymmdd_nnn.dat``` and by changing our file rotation policy, we can remove files by date as well:

1. Find a file that is over 1 week old.
2. Make sure all references to this file in the database are removed. (DELETE * FROM transactions WHERE filename = $filename;)
3. Delete the file from disk.
