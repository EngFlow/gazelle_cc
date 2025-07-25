# cc_generate directive

By defauly we should always generate cc_rules 
This behaviour can be overriden by `# gazelle:cc_generate <bool>` directive.
When disabled it should not remove existing rules even if referenced sources does not exist.
It should still be possible to index sources defined in existing rules