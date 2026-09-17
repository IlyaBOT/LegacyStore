#ifndef LSCompatibility_h
#define LSCompatibility_h

#import <Cocoa/Cocoa.h>
#import <AvailabilityMacros.h>

#if defined(MAC_OS_X_VERSION_MAX_ALLOWED) && MAC_OS_X_VERSION_MAX_ALLOWED >= 1050
typedef NSInteger LSInteger;
typedef NSUInteger LSUInteger;
#else
typedef int LSInteger;
typedef unsigned int LSUInteger;
#endif

#ifndef MAC_OS_X_VERSION_10_4
#define MAC_OS_X_VERSION_10_4 1040
#endif

#ifndef MAC_OS_X_VERSION_10_5
#define MAC_OS_X_VERSION_10_5 1050
#endif

#endif
