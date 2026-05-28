# LED Configuration
LED_COUNT = 48  # Number of LED pixels
LED_PIN = 18  # GPIO pin connected to the pixels (18 uses PWM!)
LED_FREQ_HZ = 800000  # LED signal frequency in hertz (usually 800khz)
LED_DMA = 10  # DMA channel to use for generating signal
LED_BRIGHTNESS = 255  # Set to 0 for darkest and 255 for brightest
LED_INVERT = False  # True to invert the signal
LED_CHANNEL = 0  # Set to '1' for GPIOs 13, 19, 41, 45 or 53

# Warehouse Shelf Configuration
SHELF_LAYOUT = {
    "A1": {"led_start": 0, "led_end": 5},
    "A2": {"led_start": 6, "led_end": 11},
    # Define all 48 shelf locations here
}

# Cart profile configuration
CART_PROFILES = {
    "standard": {
        "display_name": "Standard Cart",
        "row_configs": [
            {"cols": 28},
            {"cols": 28},
            {"cols": 14},
            {"cols": 14},
            {"cols": 8},
            {"cols": 8},
        ],
        # Google Sheet column index for LED mapping (A=0, B=1, C=2, ...)
        "led_column_index": None,
    },
    "large_cart": {
        "display_name": "Large Cart",
        "row_configs": [
            {"cols": 8},
            {"cols": 8},
            {"cols": 8},
            {"cols": 8},
            {"cols": 8},
            {"cols": 8},
        ],
        "led_column_index": None,
    },
}


def get_cart_profile(profile_name=None):
    """Return a resolved cart profile dictionary."""
    resolved_name = profile_name or "standard"
    profile = CART_PROFILES.get(resolved_name)
    if profile is None:
        profile = CART_PROFILES["standard"]
        resolved_name = "standard"

    # Return a shallow copy so callers can safely read and pass values around.
    return {
        "name": resolved_name,
        "display_name": profile["display_name"],
        "row_configs": list(profile["row_configs"]),
        "led_column_index": profile.get("led_column_index"),
    }


def get_available_cart_profiles():
    """Return profile names for CLI/UI validation."""
    return list(CART_PROFILES.keys())
