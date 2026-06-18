export declare const AGENT_CATALOG_VERSION = "2026-06-03.phase5";
export declare const UNIPOST_AGENT_INTENTS: readonly [{
    readonly name: "diagnose_setup";
    readonly safety_level: "read_only";
    readonly requires_user_confirmation: false;
    readonly required_user_confirmations: readonly [];
    readonly required_inputs: readonly [];
    readonly optional_inputs: readonly ["client"];
    readonly canonical_actions: readonly ["agent.bootstrap", "agent.context"];
}, {
    readonly name: "diagnose_account";
    readonly safety_level: "read_only";
    readonly requires_user_confirmation: false;
    readonly required_user_confirmations: readonly [];
    readonly required_inputs: readonly ["account_id"];
    readonly optional_inputs: readonly [];
    readonly canonical_actions: readonly ["accounts.health", "accounts.capabilities", "accounts.metrics"];
}, {
    readonly name: "create_draft_post";
    readonly safety_level: "draft_write";
    readonly requires_user_confirmation: false;
    readonly required_user_confirmations: readonly [];
    readonly required_inputs: readonly ["account_ids", "caption"];
    readonly optional_inputs: readonly ["media_ids", "platform_posts"];
    readonly canonical_actions: readonly ["posts.validate", "posts.draft"];
}, {
    readonly name: "plan_publish_post";
    readonly safety_level: "live_write_plan";
    readonly requires_user_confirmation: true;
    readonly required_user_confirmations: readonly ["approve_live_publish"];
    readonly required_inputs: readonly ["account_ids", "caption"];
    readonly optional_inputs: readonly ["scheduled_at", "media_ids", "platform_posts", "idempotency_key"];
    readonly canonical_actions: readonly ["posts.validate", "posts.create_dry_run", "posts.create"];
}, {
    readonly name: "connect_account";
    readonly safety_level: "setup_write";
    readonly requires_user_confirmation: false;
    readonly required_user_confirmations: readonly [];
    readonly required_inputs: readonly ["platform"];
    readonly optional_inputs: readonly ["profile_id", "return_url", "external_user_id", "external_user_email"];
    readonly canonical_actions: readonly ["connect.create", "connect.wait"];
}, {
    readonly name: "upload_media";
    readonly safety_level: "setup_write";
    readonly requires_user_confirmation: true;
    readonly required_user_confirmations: readonly ["approve_local_file_upload"];
    readonly required_inputs: readonly ["file_path"];
    readonly optional_inputs: readonly ["content_type"];
    readonly canonical_actions: readonly ["media.upload", "media.wait"];
}, {
    readonly name: "generate_post_example";
    readonly safety_level: "read_only";
    readonly requires_user_confirmation: false;
    readonly required_user_confirmations: readonly [];
    readonly required_inputs: readonly [];
    readonly optional_inputs: readonly ["language", "account_ids", "caption"];
    readonly canonical_actions: readonly ["examples.posts.create"];
}, {
    readonly name: "diagnose_logs";
    readonly safety_level: "read_only";
    readonly requires_user_confirmation: false;
    readonly required_user_confirmations: readonly [];
    readonly required_inputs: readonly [];
    readonly optional_inputs: readonly ["status", "category", "request_id", "since", "after_id"];
    readonly canonical_actions: readonly ["logs.list", "logs.stream"];
}, {
    readonly name: "explain_request_failure";
    readonly safety_level: "read_only";
    readonly requires_user_confirmation: false;
    readonly required_user_confirmations: readonly [];
    readonly required_inputs: readonly [];
    readonly optional_inputs: readonly ["request_id", "log_id"];
    readonly canonical_actions: readonly ["doctor.explain", "logs.get"];
}];
export declare const MCP_AGENT_TOOLS: readonly [{
    readonly name: "unipost_agent_capabilities";
    readonly description: "Return the UniPost agent intent catalog, safety levels, canonical actions, and status enums.";
}, {
    readonly name: "unipost_agent_context";
    readonly description: "Return workspace, profiles, and connected accounts for agent grounding.";
}, {
    readonly name: "unipost_agent_plan";
    readonly description: "Convert an explicit UniPost intent plus structured inputs into safe canonical actions.";
}, {
    readonly name: "unipost_debug_recent_logs";
    readonly description: "Fetch recent workspace-scoped UniPost logs for agent debugging.";
}, {
    readonly name: "unipost_debug_explain_request";
    readonly description: "Explain one UniPost log entry or request id and suggest safe next debugging actions.";
}, {
    readonly name: "unipost_debug_stream_info";
    readonly description: "Return SSE log stream connection instructions for live agent debugging.";
}];
export declare function intentByName(name: string): {
    readonly name: "diagnose_setup";
    readonly safety_level: "read_only";
    readonly requires_user_confirmation: false;
    readonly required_user_confirmations: readonly [];
    readonly required_inputs: readonly [];
    readonly optional_inputs: readonly ["client"];
    readonly canonical_actions: readonly ["agent.bootstrap", "agent.context"];
} | {
    readonly name: "diagnose_account";
    readonly safety_level: "read_only";
    readonly requires_user_confirmation: false;
    readonly required_user_confirmations: readonly [];
    readonly required_inputs: readonly ["account_id"];
    readonly optional_inputs: readonly [];
    readonly canonical_actions: readonly ["accounts.health", "accounts.capabilities", "accounts.metrics"];
} | {
    readonly name: "create_draft_post";
    readonly safety_level: "draft_write";
    readonly requires_user_confirmation: false;
    readonly required_user_confirmations: readonly [];
    readonly required_inputs: readonly ["account_ids", "caption"];
    readonly optional_inputs: readonly ["media_ids", "platform_posts"];
    readonly canonical_actions: readonly ["posts.validate", "posts.draft"];
} | {
    readonly name: "plan_publish_post";
    readonly safety_level: "live_write_plan";
    readonly requires_user_confirmation: true;
    readonly required_user_confirmations: readonly ["approve_live_publish"];
    readonly required_inputs: readonly ["account_ids", "caption"];
    readonly optional_inputs: readonly ["scheduled_at", "media_ids", "platform_posts", "idempotency_key"];
    readonly canonical_actions: readonly ["posts.validate", "posts.create_dry_run", "posts.create"];
} | {
    readonly name: "connect_account";
    readonly safety_level: "setup_write";
    readonly requires_user_confirmation: false;
    readonly required_user_confirmations: readonly [];
    readonly required_inputs: readonly ["platform"];
    readonly optional_inputs: readonly ["profile_id", "return_url", "external_user_id", "external_user_email"];
    readonly canonical_actions: readonly ["connect.create", "connect.wait"];
} | {
    readonly name: "upload_media";
    readonly safety_level: "setup_write";
    readonly requires_user_confirmation: true;
    readonly required_user_confirmations: readonly ["approve_local_file_upload"];
    readonly required_inputs: readonly ["file_path"];
    readonly optional_inputs: readonly ["content_type"];
    readonly canonical_actions: readonly ["media.upload", "media.wait"];
} | {
    readonly name: "generate_post_example";
    readonly safety_level: "read_only";
    readonly requires_user_confirmation: false;
    readonly required_user_confirmations: readonly [];
    readonly required_inputs: readonly [];
    readonly optional_inputs: readonly ["language", "account_ids", "caption"];
    readonly canonical_actions: readonly ["examples.posts.create"];
} | {
    readonly name: "diagnose_logs";
    readonly safety_level: "read_only";
    readonly requires_user_confirmation: false;
    readonly required_user_confirmations: readonly [];
    readonly required_inputs: readonly [];
    readonly optional_inputs: readonly ["status", "category", "request_id", "since", "after_id"];
    readonly canonical_actions: readonly ["logs.list", "logs.stream"];
} | {
    readonly name: "explain_request_failure";
    readonly safety_level: "read_only";
    readonly requires_user_confirmation: false;
    readonly required_user_confirmations: readonly [];
    readonly required_inputs: readonly [];
    readonly optional_inputs: readonly ["request_id", "log_id"];
    readonly canonical_actions: readonly ["doctor.explain", "logs.get"];
};
export declare function agentCapabilitiesPayload(): {
    catalog_version: string;
    status_enums: {
        post: string[];
        connect_session: string[];
        media: string[];
    };
    intents: readonly [{
        readonly name: "diagnose_setup";
        readonly safety_level: "read_only";
        readonly requires_user_confirmation: false;
        readonly required_user_confirmations: readonly [];
        readonly required_inputs: readonly [];
        readonly optional_inputs: readonly ["client"];
        readonly canonical_actions: readonly ["agent.bootstrap", "agent.context"];
    }, {
        readonly name: "diagnose_account";
        readonly safety_level: "read_only";
        readonly requires_user_confirmation: false;
        readonly required_user_confirmations: readonly [];
        readonly required_inputs: readonly ["account_id"];
        readonly optional_inputs: readonly [];
        readonly canonical_actions: readonly ["accounts.health", "accounts.capabilities", "accounts.metrics"];
    }, {
        readonly name: "create_draft_post";
        readonly safety_level: "draft_write";
        readonly requires_user_confirmation: false;
        readonly required_user_confirmations: readonly [];
        readonly required_inputs: readonly ["account_ids", "caption"];
        readonly optional_inputs: readonly ["media_ids", "platform_posts"];
        readonly canonical_actions: readonly ["posts.validate", "posts.draft"];
    }, {
        readonly name: "plan_publish_post";
        readonly safety_level: "live_write_plan";
        readonly requires_user_confirmation: true;
        readonly required_user_confirmations: readonly ["approve_live_publish"];
        readonly required_inputs: readonly ["account_ids", "caption"];
        readonly optional_inputs: readonly ["scheduled_at", "media_ids", "platform_posts", "idempotency_key"];
        readonly canonical_actions: readonly ["posts.validate", "posts.create_dry_run", "posts.create"];
    }, {
        readonly name: "connect_account";
        readonly safety_level: "setup_write";
        readonly requires_user_confirmation: false;
        readonly required_user_confirmations: readonly [];
        readonly required_inputs: readonly ["platform"];
        readonly optional_inputs: readonly ["profile_id", "return_url", "external_user_id", "external_user_email"];
        readonly canonical_actions: readonly ["connect.create", "connect.wait"];
    }, {
        readonly name: "upload_media";
        readonly safety_level: "setup_write";
        readonly requires_user_confirmation: true;
        readonly required_user_confirmations: readonly ["approve_local_file_upload"];
        readonly required_inputs: readonly ["file_path"];
        readonly optional_inputs: readonly ["content_type"];
        readonly canonical_actions: readonly ["media.upload", "media.wait"];
    }, {
        readonly name: "generate_post_example";
        readonly safety_level: "read_only";
        readonly requires_user_confirmation: false;
        readonly required_user_confirmations: readonly [];
        readonly required_inputs: readonly [];
        readonly optional_inputs: readonly ["language", "account_ids", "caption"];
        readonly canonical_actions: readonly ["examples.posts.create"];
    }, {
        readonly name: "diagnose_logs";
        readonly safety_level: "read_only";
        readonly requires_user_confirmation: false;
        readonly required_user_confirmations: readonly [];
        readonly required_inputs: readonly [];
        readonly optional_inputs: readonly ["status", "category", "request_id", "since", "after_id"];
        readonly canonical_actions: readonly ["logs.list", "logs.stream"];
    }, {
        readonly name: "explain_request_failure";
        readonly safety_level: "read_only";
        readonly requires_user_confirmation: false;
        readonly required_user_confirmations: readonly [];
        readonly required_inputs: readonly [];
        readonly optional_inputs: readonly ["request_id", "log_id"];
        readonly canonical_actions: readonly ["doctor.explain", "logs.get"];
    }];
    mcp_tools: readonly [{
        readonly name: "unipost_agent_capabilities";
        readonly description: "Return the UniPost agent intent catalog, safety levels, canonical actions, and status enums.";
    }, {
        readonly name: "unipost_agent_context";
        readonly description: "Return workspace, profiles, and connected accounts for agent grounding.";
    }, {
        readonly name: "unipost_agent_plan";
        readonly description: "Convert an explicit UniPost intent plus structured inputs into safe canonical actions.";
    }, {
        readonly name: "unipost_debug_recent_logs";
        readonly description: "Fetch recent workspace-scoped UniPost logs for agent debugging.";
    }, {
        readonly name: "unipost_debug_explain_request";
        readonly description: "Explain one UniPost log entry or request id and suggest safe next debugging actions.";
    }, {
        readonly name: "unipost_debug_stream_info";
        readonly description: "Return SSE log stream connection instructions for live agent debugging.";
    }];
};
export declare function planForIntent(input: Record<string, unknown>): {
    intent: "diagnose_setup" | "diagnose_account" | "create_draft_post" | "plan_publish_post" | "connect_account" | "upload_media" | "generate_post_example" | "diagnose_logs" | "explain_request_failure";
    safety_level: "read_only" | "draft_write" | "live_write_plan" | "setup_write";
    missing_inputs: ("account_id" | "account_ids" | "caption" | "platform" | "file_path")[];
    required_user_confirmations: ("approve_live_publish" | "approve_local_file_upload")[];
    safe_to_execute_without_user: boolean;
    actions: {
        canonical_action: "agent.bootstrap" | "agent.context" | "accounts.health" | "accounts.capabilities" | "accounts.metrics" | "posts.validate" | "posts.draft" | "posts.create_dry_run" | "posts.create" | "connect.create" | "connect.wait" | "media.upload" | "media.wait" | "examples.posts.create" | "logs.list" | "logs.stream" | "doctor.explain" | "logs.get";
        safety_level: "read_only" | "draft_write" | "live_write_plan" | "setup_write";
    }[];
};
export declare function registerAgentContractTools(server: any, apiRequest: (path: string, options?: RequestInit) => Promise<any>, options?: {
    apiUrl?: string;
}): void;
