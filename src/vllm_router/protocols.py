import logging
import time
from typing import List, Optional

try:
    # Pydantic v2
    from pydantic import BaseModel, ConfigDict, Field, model_validator
    PYDANTIC_V2 = True
except ImportError:
    # Pydantic v1
    from pydantic import BaseModel, Field
    from pydantic.config import ConfigDict
    from pydantic.class_validators import validator
    PYDANTIC_V2 = False


logger = logging.getLogger(__name__)


class OpenAIBaseModel(BaseModel):
    # OpenAI API does allow extra fields
    model_config = ConfigDict(extra="allow") if PYDANTIC_V2 else {
        "extra": "allow"
    }

    if PYDANTIC_V2:
        @model_validator(mode="before")
        @classmethod
        def __log_extra_fields__(cls, data):
            if isinstance(data, dict):
                # Get all class field names and their potential aliases
                field_names = set()
                for field_name, field in cls.model_fields.items():
                    field_names.add(field_name)
                    if hasattr(field, "alias") and field.alias:
                        field_names.add(field.alias)

                # Compare against both field names and aliases
                extra_fields = data.keys() - field_names
                if extra_fields:
                    logger.warning(
                        "The following fields were present in the request "
                        "but ignored: %s",
                        extra_fields,
                    )
            return data
    else:
        @validator("*", pre=True)
        def log_extra_fields(cls, v, values, **kwargs):
            field_names = set(cls.__fields__.keys())
            extra_fields = values.keys() - field_names
            if extra_fields:
                logger.warning(
                    "The following fields were present in the request "
                    "but ignored: %s",
                    extra_fields,
                )
            return v


class ErrorResponse(OpenAIBaseModel):
    object: str = "error"
    message: str
    type: str
    param: Optional[str] = None
    code: int


class ModelCard(OpenAIBaseModel):
    id: str
    object: str = "model"
    created: int = Field(default_factory=lambda: int(time.time()))
    owned_by: str = "vllm"
    root: Optional[str] = None


class ModelList(OpenAIBaseModel):
    object: str = "list"
    data: List[ModelCard] = Field(default_factory=list)
