"use client";

import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";

export default function AccountsPage() {
  return (
    <div>
      <h1 className="text-2xl font-bold mb-6">Connected Accounts</h1>
      <Card>
        <CardContent className="flex flex-col items-center justify-center py-16">
          <div className="text-4xl mb-4">🔗</div>
          <p className="font-medium mb-1">No accounts connected yet</p>
          <p className="text-sm text-muted-foreground mb-6">
            Connect your first social account to start posting.
          </p>
          <Button disabled>
            Connect Account
          </Button>
          <p className="text-xs text-muted-foreground mt-2">Coming soon</p>
        </CardContent>
      </Card>
    </div>
  );
}
