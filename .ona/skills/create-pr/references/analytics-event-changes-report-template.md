## 📊 Analytics Events Changes

### ✨ New Events
- **`event_name`** (Frontend/Backend/Mobile)
  - **Trigger:** When user performs [specific action] in [specific context]
  - **Business Purpose:** [Why we need this data - business/product justification]
  - **Properties:**
    - `property_name` (string, required) - Description with all possible values
    - `user_id` (uuid, required) - Always include for user attribution
    - `timestamp` (iso8601, auto) - Event occurrence time
  - **Frequency:** [Expected volume - per user session, daily, etc.]

### 🔄 Modified Events  
- **`existing_event_name`**
  - **Changes Made:**
    - ✅ **Added:** `new_property` (type, constraint) - Purpose and description
    - ❌ **Removed:** `deprecated_property` - Removal reason and migration path
    - 🔄 **Modified:** `existing_property` - Old format → New format (why changed)

### 🗑️ Removed Events
- **`deprecated_event`** 
  - **Removal Reason:** [Technical or business justification]
