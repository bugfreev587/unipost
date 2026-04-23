export type Platform =
  | "twitter"
  | "linkedin"
  | "instagram"
  | "threads"
  | "tiktok"
  | "youtube"
  | "bluesky";

export type AccountStatus = "active" | "reconnect_required" | "disconnected";
export type ConnectionType = "byo" | "managed";

export interface SocialAccount {
  id: string;
  platform: Platform;
  account_name: string | null;
  external_user_id?: string;
  external_user_email?: string;
  connected_at?: string;
  status: AccountStatus;
  connection_type?: ConnectionType;
}

export interface AccountHealth {
  account_id: string;
  status: "ok" | "degraded" | "disconnected";
  last_checked_at?: string;
  error?: string;
}

export interface ListAccountsParams {
  platform?: Platform;
  externalUserId?: string;
  status?: AccountStatus;
  profileId?: string;
}

export type PostStatus =
  | "draft"
  | "scheduled"
  | "queued"
  | "publishing"
  | "dispatching"
  | "retrying"
  | "processing"
  | "published"
  | "partial"
  | "failed"
  | "cancelled";

export interface PlatformResult {
  id?: string;
  social_account_id: string;
  platform?: Platform | string;
  account_name?: string;
  caption?: string;
  status: string;
  external_id?: string;
  url?: string;
  error_message?: string;
  published_at?: string;
  warnings?: string[];
  submitted?: Record<string, unknown>;
}

export interface Post {
  id: string;
  caption: string | null;
  media_urls?: string[];
  status: PostStatus | string;
  execution_mode?: string;
  queued_results_count?: number;
  active_job_count?: number;
  retrying_count?: number;
  dead_count?: number;
  created_at: string;
  scheduled_at?: string;
  published_at?: string;
  results?: PlatformResult[];
}

export interface CreatePostPlatformPost {
  accountId: string;
  caption?: string;
  mediaUrls?: string[];
  mediaIds?: string[];
  threadPosition?: number;
  firstComment?: string;
  inReplyTo?: string;
  platformOptions?: Record<string, unknown>;
}

export interface CreatePostParams {
  caption?: string;
  accountIds?: string[];
  mediaUrls?: string[];
  mediaIds?: string[];
  scheduledAt?: string;
  status?: "draft";
  idempotencyKey?: string;
  platformPosts?: CreatePostPlatformPost[];
}

export interface ListPostsParams {
  status?: PostStatus;
  platform?: Platform;
  from?: string;
  to?: string;
  limit?: number;
  cursor?: string;
}

export interface DeliveryJob {
  id: string;
  post_id: string;
  social_post_result_id: string;
  social_account_id: string;
  platform: string;
  kind: string;
  state: string;
  attempts: number;
  max_attempts: number;
  failure_stage?: string;
  error_code?: string;
  platform_error_code?: string;
  last_error?: string;
  next_run_at?: string;
  last_attempt_at?: string;
  created_at: string;
  updated_at: string;
}

export interface PostQueueSnapshot {
  post: Post;
  jobs: DeliveryJob[];
}

export interface PostAnalytics {
  post_id: string;
  impressions: number;
  engagements: number;
  likes: number;
  comments: number;
  shares: number;
  clicks: number;
  results?: Record<string, Record<string, number>>;
}

export type WebhookEventType =
  | "post.published"
  | "post.partial"
  | "post.failed"
  | "post.scheduled"
  | "account.connected"
  | "account.disconnected"
  | "account.refreshed"
  | "account.quota_warning"
  | "account.quota_exceeded";

export interface WebhookEvent<TData = Record<string, unknown>> {
  event: WebhookEventType | string;
  timestamp: string;
  data: TData;
}

export interface VerifyWebhookOptions {
  payload: string | Uint8Array | Buffer;
  signature?: string | null;
  secret: string;
}

export interface WebhookSubscription {
  id: string;
  url: string;
  events: string[];
  active: boolean;
  secret_preview: string;
  created_at: string;
}

export interface WebhookSubscriptionSecret extends WebhookSubscription {
  secret: string;
}

export interface CreateWebhookParams {
  url: string;
  events: string[];
}

export interface UpdateWebhookParams {
  url?: string;
  events?: string[];
  active?: boolean;
}

export interface ConnectSession {
  id: string;
  url: string;
  status: "pending" | "completed" | "expired";
  expires_at: string;
  platform: string;
  external_user_id: string;
}

export interface CreateConnectSessionParams {
  platform: string;
  profileId?: string;
  externalUserId: string;
  externalUserEmail?: string;
  returnUrl?: string;
}

export interface ManagedUser {
  external_user_id: string;
  external_user_email?: string;
  account_count?: number;
  platform_counts?: Record<string, number>;
  reconnect_count?: number;
  accounts?: SocialAccount[];
  created_at?: string;
}

export interface MediaUploadRequest {
  filename: string;
  contentType: string;
  sizeBytes: number;
  contentHash?: string;
}

export interface MediaUploadResponse {
  media_id: string;
  mediaId: string;
  upload_url: string;
  uploadUrl: string;
  status: string;
  expires_at?: string;
}

export interface MediaObject {
  id: string;
  status: string;
  content_type: string;
  size_bytes: number;
  upload_url?: string;
  download_url?: string;
  expires_at?: string;
  created_at?: string;
}

export type Granularity = "day" | "week" | "month";
export type GroupBy = "platform" | "social_account_id" | "status" | "external_user_id";

export interface AnalyticsRollupParams {
  from: string;
  to: string;
  granularity?: Granularity;
  groupBy?: GroupBy;
}

export interface AnalyticsBucket {
  key: string;
  impressions: number;
  engagements: number;
  likes: number;
  comments: number;
  shares: number;
  clicks: number;
}

export interface AnalyticsRollup {
  from: string;
  to: string;
  granularity: Granularity;
  buckets: AnalyticsBucket[];
}

export interface PaginatedResponse<T> {
  data: T[];
  nextCursor?: string;
  meta?: {
    total?: number;
    limit?: number;
    has_more?: boolean;
    next_cursor?: string;
  };
}

export interface UniPostClientOptions {
  apiKey?: string;
  baseUrl?: string;
  timeout?: number;
}

declare class Accounts {
  constructor(http: unknown);
  list(params?: ListAccountsParams): Promise<PaginatedResponse<SocialAccount>>;
  get(accountId: string): Promise<SocialAccount>;
  health(accountId: string): Promise<AccountHealth>;
}

declare class Posts {
  constructor(http: unknown);
  create(params: CreatePostParams): Promise<Post>;
  list(params?: ListPostsParams): Promise<PaginatedResponse<Post> & { nextCursor?: string }>;
  listAll(params?: Omit<ListPostsParams, "cursor">): AsyncGenerator<Post>;
  get(postId: string): Promise<Post>;
  getQueue(postId: string): Promise<PostQueueSnapshot>;
  analytics(postId: string): Promise<PostAnalytics>;
  publish(postId: string): Promise<Post>;
  cancel(postId: string): Promise<Post>;
  retryResult(postId: string, resultId: string): Promise<DeliveryJob>;
  bulkCreate(posts: CreatePostParams[]): Promise<Post[]>;
}

declare class Media {
  constructor(http: unknown);
  upload(params: MediaUploadRequest): Promise<MediaUploadResponse>;
  get(mediaId: string): Promise<MediaObject>;
  delete(mediaId: string): Promise<Record<string, unknown> | undefined>;
  uploadFile(filePath: string): Promise<string>;
}

declare class Analytics {
  constructor(http: unknown);
  rollup(params: AnalyticsRollupParams): Promise<AnalyticsRollup>;
}

declare class Connect {
  constructor(http: unknown);
  createSession(params: CreateConnectSessionParams): Promise<ConnectSession>;
  getSession(sessionId: string): Promise<ConnectSession>;
}

declare class Users {
  constructor(http: unknown);
  list(): Promise<PaginatedResponse<ManagedUser>>;
  get(externalUserId: string): Promise<ManagedUser>;
}

declare class Webhooks {
  constructor(http: unknown);
  create(params: CreateWebhookParams): Promise<WebhookSubscriptionSecret>;
  list(): Promise<PaginatedResponse<WebhookSubscription>>;
  get(webhookId: string): Promise<WebhookSubscription>;
  update(webhookId: string, params: UpdateWebhookParams): Promise<WebhookSubscription>;
  rotate(webhookId: string): Promise<WebhookSubscriptionSecret>;
  delete(webhookId: string): Promise<Record<string, unknown> | undefined>;
}

declare class UniPost {
  readonly accounts: Accounts;
  readonly posts: Posts;
  readonly media: Media;
  readonly analytics: Analytics;
  readonly connect: Connect;
  readonly users: Users;
  readonly webhooks: Webhooks;
  constructor(options?: UniPostClientOptions);
}

declare class UniPostError extends Error {
  readonly status: number;
  readonly code: string;
  constructor(message: string, status: number, code: string);
}

declare class AuthError extends UniPostError {
  constructor(message?: string);
}

declare class NotFoundError extends UniPostError {
  constructor(message?: string);
}

declare class ValidationError extends UniPostError {
  readonly errors: Record<string, string[]>;
  constructor(message?: string, errors?: Record<string, string[]>);
}

declare class RateLimitError extends UniPostError {
  readonly retryAfter: number;
  constructor(retryAfter: number, message?: string);
}

declare class PlatformError extends UniPostError {
  readonly platform: string;
  constructor(message: string, platform: string);
}

declare class QuotaError extends UniPostError {
  constructor(message?: string);
}

declare function verifyWebhookSignature(options: VerifyWebhookOptions): Promise<boolean>;

export {
  Accounts,
  Analytics,
  AuthError,
  Connect,
  Media,
  NotFoundError,
  PlatformError,
  Posts,
  QuotaError,
  RateLimitError,
  UniPost,
  UniPostError,
  Users,
  ValidationError,
  Webhooks,
  verifyWebhookSignature,
};
