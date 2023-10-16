//
//  CMDynamicPropertyProvider.m
//
//
//  Created by Steve Cosman on 2023-05-22.
//

#import "CMBaseDynamicPropertyProvider.h"

@import Appcore;

@interface CMDynamicPropertyProviderWrapper ()
@property(nonnull, strong) id<CMDynamicPropertyProvider> pp;
@end

@implementation CMDynamicPropertyProviderWrapper

- (instancetype)initWithPP:(id<CMDynamicPropertyProvider>)pp {
    self = [super init];
    if (self) {
        self.pp = pp;
    }
    return self;
}

- (BOOL)boolValue {
    if ([_pp respondsToSelector:@selector(boolValue)]) {
        return self.pp.boolValue;
    }
    return false;
}

- (double)floatValue {
    if ([_pp respondsToSelector:@selector(floatValue)]) {
        return self.pp.floatValue;
    } else if ([_pp respondsToSelector:@selector(nillableFloatValue)]) {
        NSNumber *v = self.pp.nillableFloatValue;
        if (v) {
            return v.doubleValue;
        }
    }
    return AppcoreLibPropertyProviderNilFloatValue;
}

- (int64_t)intValue {
    if ([_pp respondsToSelector:@selector(intValue)]) {
        return self.pp.intValue;
    } else if ([_pp respondsToSelector:@selector(nillableIntValue)]) {
        NSNumber *v = self.pp.nillableIntValue;
        if (v) {
            return v.longLongValue;
        }
    }
    return AppcoreLibPropertyProviderNilIntValue;
}

- (NSString *_Nonnull)stringValue {
    if ([_pp respondsToSelector:@selector(stringValue)]) {
        NSString *v = self.pp.stringValue;
        if (v) {
            return v;
        }
    }
    return AppcoreLibPropertyProviderNilStringValue;
}

- (long)type {
    return self.pp.type;
}

@end
