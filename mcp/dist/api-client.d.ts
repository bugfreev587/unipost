export declare class UniPostApiError extends Error {
    status: number;
    code: string;
    normalizedCode: string;
    requestId: string;
    issues: any[];
    body: any;
    constructor(args: {
        status: number;
        message: string;
        code?: string;
        normalizedCode?: string;
        requestId?: string;
        issues?: any[];
        body?: any;
    });
}
export declare function canonicalizeApiPath(path: string): string;
export declare function apiRequest(apiUrl: string, path: string, apiKey: string, options?: RequestInit): Promise<any>;
