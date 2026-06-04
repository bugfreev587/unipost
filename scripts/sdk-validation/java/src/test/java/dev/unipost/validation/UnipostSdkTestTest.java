package dev.unipost.validation;

import org.junit.jupiter.api.Test;

import java.util.List;
import java.util.Map;

import static org.junit.jupiter.api.Assertions.assertEquals;

final class UnipostSdkTestTest {
    @Test
    void scheduledContentUpdateBodyIncludesTargetAccount() {
        Map<String, Object> body = UnipostSdkTest.scheduledContentUpdateBody(
                "Updated scheduled caption",
                "acct_123",
                "2026-06-05T12:00:00Z"
        );

        assertEquals("Updated scheduled caption", body.get("caption"));
        assertEquals(List.of("acct_123"), body.get("account_ids"));
        assertEquals("2026-06-05T12:00:00Z", body.get("scheduled_at"));
    }
}
