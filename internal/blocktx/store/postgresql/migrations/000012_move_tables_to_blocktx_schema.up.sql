ALTER TABLE public.block_processing SET SCHEMA blocktx;
ALTER TABLE public.block_transactions_map SET SCHEMA blocktx;
ALTER TABLE public.blocks SET SCHEMA blocktx;
ALTER TABLE public.primary_blocktx SET SCHEMA blocktx;
ALTER TABLE public.transactions SET SCHEMA blocktx;
SET search_path TO blocktx;
